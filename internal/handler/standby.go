/**
 * 备用账号池管理接口
 *
 * 路由：
 *   GET  /admin/standby/state                      — 协调器状态 + 备用池账号列表与统计
 *   POST /admin/standby/accounts/ingest            — 导入账号到备用池（仅校验格式，不校验账号可用性）
 *   POST /admin/standby/accounts/export            — 导出备用池账号（sub2api 格式）
 *   POST /admin/standby/accounts/delete            — 删除备用池单账号
 *   POST /admin/standby/accounts/toggle-enabled    — 启用/停用备用池单账号
 *   POST /admin/standby/health-check               — 手动触发一次性健康检查（SSE 流式进度）
 */
package handler

import (
	"encoding/json"
	"strings"

	"codex-proxy/internal/auth"

	"github.com/valyala/fasthttp"
)

/* standbyManager 安全获取备用池 Manager；未配置时为 nil */
func (h *ProxyHandler) standbyManager() *auth.Manager {
	if h == nil || h.standbyCtrl == nil {
		return nil
	}
	return h.standbyCtrl.Standby()
}

/**
 * handleStandbyState GET /admin/standby/state
 * 返回 {active, primary_total, standby_total, note, summary, accounts, recent_events}
 */
func (h *ProxyHandler) handleStandbyState(ctx *fasthttp.RequestCtx) {
	if h.standbyCtrl == nil {
		writeJSON(ctx, fasthttp.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{"message": "备用账号池未启用", "type": "invalid_request_error"},
		})
		return
	}

	args := ctx.QueryArgs()
	query := strings.ToLower(strings.TrimSpace(string(args.Peek("q"))))
	statusFilter := strings.ToLower(strings.TrimSpace(string(args.Peek("status"))))
	statusFilterEnabled := statusFilter == "enabled"
	statusFilterDisabled := statusFilter == "disabled"

	mgr := h.standbyManager()
	var accountStats []auth.AccountStats
	active, cooldown, disabled, refreshDisabledCount := 0, 0, 0, 0
	var totalInputTokens, totalOutputTokens int64
	if mgr != nil {
		accounts := mgr.GetAccounts()
		accountStats = make([]auth.AccountStats, 0, len(accounts))
		for _, acc := range accounts {
			s := acc.GetStats()
			totalInputTokens += s.Usage.InputTokens
			totalOutputTokens += s.Usage.OutputTokens
			if s.RefreshDisabled {
				refreshDisabledCount++
			}
			switch s.Status {
			case "active":
				active++
			case "cooldown":
				cooldown++
			case "disabled":
				disabled++
			}
			if statusFilterEnabled && s.Status == "disabled" {
				continue
			}
			if statusFilterDisabled && s.Status != "disabled" {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(s.Email), query) {
				continue
			}
			accountStats = append(accountStats, s)
		}
	}

	snap := h.standbyCtrl.Snapshot()
	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"active":        snap.Active,
		"primary_total": snap.PrimaryTotal,
		"standby_total": snap.StandbyTotal,
		"note":          snap.Note,
		"summary": map[string]any{
			"total":               snap.StandbyTotal,
			"active":              active,
			"cooldown":            cooldown,
			"disabled":            disabled,
			"refresh_disabled":    refreshDisabledCount,
			"total_input_tokens":  totalInputTokens,
			"total_output_tokens": totalOutputTokens,
		},
		"accounts": accountStats,
	})
}

/**
 * handleStandbyAccountsIngest POST /admin/standby/accounts/ingest
 * 复用主池的 IngestAccountsFromJSON：仅做 JSON/NDJSON 解析与字段必填校验，不发起任何上游请求
 */
func (h *ProxyHandler) handleStandbyAccountsIngest(ctx *fasthttp.RequestCtx) {
	mgr := h.standbyManager()
	if mgr == nil {
		writeJSON(ctx, fasthttp.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{"message": "备用账号池未启用", "type": "invalid_request_error"},
		})
		return
	}
	if !ctx.IsPost() {
		writeJSON(ctx, fasthttp.StatusMethodNotAllowed, map[string]any{
			"error": map[string]string{"message": "请使用 POST 上传"},
		})
		return
	}
	body := ctx.PostBody()
	res, err := mgr.IngestAccountsFromJSON(body)
	if err != nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]string{"message": err.Error()},
		})
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, res)
}

/**
 * handleStandbyAccountsExport POST /admin/standby/accounts/export
 * 复用主池导出逻辑，但来源为备用池 Manager
 */
func (h *ProxyHandler) handleStandbyAccountsExport(ctx *fasthttp.RequestCtx) {
	mgr := h.standbyManager()
	if mgr == nil {
		writeJSON(ctx, fasthttp.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{"message": "备用账号池未启用", "type": "invalid_request_error"},
		})
		return
	}
	body := ctx.PostBody()
	if len(body) == 0 {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "请求体不能为空", "type": "invalid_request_error"},
		})
		return
	}
	var req struct {
		Emails []string `json:"emails"`
		Format string   `json:"format"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "JSON 解析失败", "type": "invalid_request_error"},
		})
		return
	}
	emails := make([]string, 0, len(req.Emails))
	for _, email := range req.Emails {
		if trimmed := strings.TrimSpace(email); trimmed != "" {
			emails = append(emails, trimmed)
		}
	}
	if len(emails) == 0 {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "请提供至少一个 email", "type": "invalid_request_error"},
		})
		return
	}
	format := strings.TrimSpace(req.Format)
	if format == "" {
		format = auth.Sub2APIExportFormatExport
	}
	if format != auth.Sub2APIExportFormatExport && format != auth.Sub2APIExportFormatArray {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "format 仅支持 sub2api-export 或 sub2api-array", "type": "invalid_request_error"},
		})
		return
	}
	result := mgr.ExportAccountsSub2API(emails)
	if result.Exported == 0 {
		writeJSON(ctx, fasthttp.StatusNotFound, map[string]any{
			"error":     map[string]any{"message": "没有可导出的账号", "type": "invalid_request_error"},
			"format":    format,
			"exported":  0,
			"not_found": result.NotFound,
			"failed":    result.Failed,
		})
		return
	}
	var data any
	switch format {
	case auth.Sub2APIExportFormatArray:
		data = result.Accounts
	default:
		data = auth.BuildSub2APIExportFile(result.Accounts, result.ExportedAt)
	}
	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"format":    format,
		"exported":  result.Exported,
		"not_found": result.NotFound,
		"failed":    result.Failed,
		"data":      data,
	})
}

/**
 * handleStandbyAccountDelete POST /admin/standby/accounts/delete
 */
func (h *ProxyHandler) handleStandbyAccountDelete(ctx *fasthttp.RequestCtx) {
	mgr := h.standbyManager()
	if mgr == nil {
		writeJSON(ctx, fasthttp.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{"message": "备用账号池未启用", "type": "invalid_request_error"},
		})
		return
	}
	body := ctx.PostBody()
	if len(body) == 0 {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "请求体不能为空", "type": "invalid_request_error"},
		})
		return
	}
	var req struct {
		Email    string `json:"email"`
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "JSON 解析失败", "type": "invalid_request_error"},
		})
		return
	}
	email := strings.TrimSpace(req.Email)
	filePath := strings.TrimSpace(req.FilePath)
	if email == "" && filePath == "" {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "请提供 email 或 file_path", "type": "invalid_request_error"},
		})
		return
	}
	result, err := mgr.RemoveAccountByIdentifier(email, filePath, auth.ReasonManualDelete)
	if err != nil {
		writeJSON(ctx, fasthttp.StatusNotFound, map[string]any{
			"error": map[string]any{"message": err.Error(), "type": "invalid_request_error"},
		})
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"email":      result.Email,
		"file_path":  result.FilePath,
		"deleted":    true,
		"pool_total": result.PoolTotal,
	})
}

/**
 * handleStandbyAccountToggleEnabled POST /admin/standby/accounts/toggle-enabled
 */
func (h *ProxyHandler) handleStandbyAccountToggleEnabled(ctx *fasthttp.RequestCtx) {
	mgr := h.standbyManager()
	if mgr == nil {
		writeJSON(ctx, fasthttp.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{"message": "备用账号池未启用", "type": "invalid_request_error"},
		})
		return
	}
	body := ctx.PostBody()
	if len(body) == 0 {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "请求体不能为空", "type": "invalid_request_error"},
		})
		return
	}
	var req struct {
		Email    string `json:"email"`
		FilePath string `json:"file_path"`
		Enabled  *bool  `json:"enabled"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "JSON 解析失败", "type": "invalid_request_error"},
		})
		return
	}
	if req.Enabled == nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "缺少 enabled 字段", "type": "invalid_request_error"},
		})
		return
	}
	email := strings.TrimSpace(req.Email)
	filePath := strings.TrimSpace(req.FilePath)
	if email == "" && filePath == "" {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "请提供 email 或 file_path", "type": "invalid_request_error"},
		})
		return
	}
	acc, err := mgr.SetAccountEnabled(email, filePath, *req.Enabled)
	if err != nil {
		writeJSON(ctx, fasthttp.StatusNotFound, map[string]any{
			"error": map[string]any{"message": err.Error(), "type": "invalid_request_error"},
		})
		return
	}
	stats := acc.GetStats()
	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"email":          stats.Email,
		"enabled":        stats.Status != "disabled",
		"status":         stats.Status,
		"disable_reason": stats.DisableReason,
	})
}

/**
 * handleStandbyHealthCheck POST /admin/standby/health-check
 * 手动一次性扫描备用池全部账号；SSE 流式返回进度
 */
func (h *ProxyHandler) handleStandbyHealthCheck(ctx *fasthttp.RequestCtx) {
	mgr := h.standbyManager()
	if mgr == nil || h.standbyHealthChecker == nil {
		writeJSON(ctx, fasthttp.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{"message": "备用账号池未启用或健康检查器未配置", "type": "invalid_request_error"},
		})
		return
	}
	ch := mgr.RunHealthCheckOnce(ctx, h.standbyHealthChecker)
	writeSSEProgress(ctx, ch)
}

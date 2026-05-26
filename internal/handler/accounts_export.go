package handler

import (
	"encoding/json"
	"strings"

	"github.com/valyala/fasthttp"

	"codex-proxy/internal/auth"
)

/**
 * handleAccountsExport POST /admin/accounts/export
 * 将选中账号导出为 sub2api 格式 JSON
 */
func (h *ProxyHandler) handleAccountsExport(ctx *fasthttp.RequestCtx) {
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

	result := h.manager.ExportAccountsSub2API(emails)
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

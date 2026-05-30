package handler

import (
	"encoding/json"

	"github.com/valyala/fasthttp"
)

type apiDebugConfigRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *ProxyHandler) handleAPIDebugConfig(ctx *fasthttp.RequestCtx) {
	if h.apiDebug == nil {
		writeJSON(ctx, fasthttp.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{"message": "API 调试服务未初始化", "type": "server_error"},
		})
		return
	}
	switch string(ctx.Method()) {
	case fasthttp.MethodGet:
		writeJSON(ctx, fasthttp.StatusOK, map[string]any{
			"object": "api_debug_config",
			"config": h.apiDebug.Config(),
		})
	case fasthttp.MethodPut:
		h.handleAPIDebugConfigSave(ctx)
	default:
		writeJSON(ctx, fasthttp.StatusMethodNotAllowed, map[string]any{
			"error": map[string]any{"message": "方法不允许", "type": "invalid_request_error"},
		})
	}
}

func (h *ProxyHandler) handleAPIDebugConfigSave(ctx *fasthttp.RequestCtx) {
	body := ctx.PostBody()
	if len(body) == 0 {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "请求体不能为空", "type": "invalid_request_error"},
		})
		return
	}
	var req apiDebugConfigRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "JSON 解析失败", "type": "invalid_request_error"},
		})
		return
	}
	if err := h.apiDebug.SetEnabled(req.Enabled); err != nil {
		writeJSON(ctx, fasthttp.StatusInternalServerError, map[string]any{
			"error": map[string]any{"message": err.Error(), "type": "server_error"},
		})
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"object": "api_debug_config",
		"config": h.apiDebug.Config(),
	})
}

func (h *ProxyHandler) handleAPIDebugTraces(ctx *fasthttp.RequestCtx) {
	if h.apiDebug == nil {
		writeJSON(ctx, fasthttp.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{"message": "API 调试服务未初始化", "type": "server_error"},
		})
		return
	}
	if string(ctx.Method()) != fasthttp.MethodGet {
		writeJSON(ctx, fasthttp.StatusMethodNotAllowed, map[string]any{
			"error": map[string]any{"message": "方法不允许", "type": "invalid_request_error"},
		})
		return
	}
	traces := h.apiDebug.List()
	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"object": "api_debug_traces",
		"traces": traces,
		"count":  len(traces),
	})
}

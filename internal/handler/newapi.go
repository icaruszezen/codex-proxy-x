package handler

import (
	"context"
	"encoding/json"
	"errors"

	"codex-proxy/internal/notify"

	"github.com/valyala/fasthttp"
)

type newAPIConfigRequest struct {
	AutoSwitch  bool   `json:"auto_switch"`
	BaseURL     string `json:"base_url"`
	AdminToken  string `json:"admin_token"`
	AdminUserID int    `json:"admin_user_id"`
	ChannelID   int    `json:"channel_id"`
	TimeoutSec  int    `json:"timeout_sec"`
}

func (h *ProxyHandler) handleNewAPIConfig(ctx *fasthttp.RequestCtx) {
	if h.newapiService == nil {
		writeJSON(ctx, fasthttp.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{"message": "NewAPI 服务未初始化", "type": "server_error"},
		})
		return
	}
	switch string(ctx.Method()) {
	case fasthttp.MethodGet:
		writeJSON(ctx, fasthttp.StatusOK, map[string]any{
			"object": "newapi_config",
			"config": h.newapiService.PublicConfig(),
		})
	case fasthttp.MethodPut:
		h.handleNewAPIConfigSave(ctx)
	default:
		writeJSON(ctx, fasthttp.StatusMethodNotAllowed, map[string]any{
			"error": map[string]any{"message": "方法不允许", "type": "invalid_request_error"},
		})
	}
}

func (h *ProxyHandler) handleNewAPIConfigSave(ctx *fasthttp.RequestCtx) {
	body := ctx.PostBody()
	if len(body) == 0 {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "请求体不能为空", "type": "invalid_request_error"},
		})
		return
	}
	var req newAPIConfigRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "JSON 解析失败", "type": "invalid_request_error"},
		})
		return
	}
	cfg := notify.NewAPIConfig{
		AutoSwitch:  req.AutoSwitch,
		BaseURL:     req.BaseURL,
		AdminToken:  req.AdminToken,
		AdminUserID: req.AdminUserID,
		ChannelID:   req.ChannelID,
		TimeoutSec:  req.TimeoutSec,
	}
	if cfg.AdminToken == "" {
		cfg.AdminToken = h.newapiService.Config().AdminToken
	}
	if err := h.newapiService.Save(cfg); err != nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": err.Error(), "type": "invalid_request_error"},
		})
		return
	}
	if cfg.AutoSwitch {
		h.syncPrimaryAvailabilityForNewAPI()
	}
	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"object": "newapi_config",
		"config": h.newapiService.PublicConfig(),
	})
}

func (h *ProxyHandler) handleNewAPITestEnable(ctx *fasthttp.RequestCtx) {
	h.handleNewAPITest(ctx, true)
}

func (h *ProxyHandler) handleNewAPITestDisable(ctx *fasthttp.RequestCtx) {
	h.handleNewAPITest(ctx, false)
}

func (h *ProxyHandler) handleNewAPITest(ctx *fasthttp.RequestCtx, enable bool) {
	if h.newapiService == nil {
		writeJSON(ctx, fasthttp.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{"message": "NewAPI 服务未初始化", "type": "server_error"},
		})
		return
	}
	var (
		result *notify.NewAPIResult
		err    error
	)
	if enable {
		result, err = h.newapiService.TestEnableChannel(context.Background())
	} else {
		result, err = h.newapiService.TestDisableChannel(context.Background())
	}
	if err != nil {
		status := fasthttp.StatusBadGateway
		errorType := "newapi_error"
		if errors.Is(err, notify.ErrNewAPIConfigIncomplete) {
			status = fasthttp.StatusBadRequest
			errorType = "invalid_request_error"
		}
		writeJSON(ctx, status, map[string]any{
			"object":  "newapi_test_result",
			"success": false,
			"result":  result,
			"error":   map[string]any{"message": err.Error(), "type": errorType},
		})
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"object":  "newapi_test_result",
		"success": true,
		"result":  result,
	})
}

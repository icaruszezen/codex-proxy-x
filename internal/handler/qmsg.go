package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"codex-proxy/internal/notify"

	"github.com/valyala/fasthttp"
)

type qmsgConfigRequest struct {
	Enabled          bool   `json:"enabled"`
	Key              string `json:"key"`
	QQ               string `json:"qq"`
	Bot              string `json:"bot"`
	TimeoutSec       int    `json:"timeout_sec"`
	MessageTemplate  string `json:"message_template"`
	EndpointTemplate string `json:"endpoint_template"`
}

type qmsgTestRequest struct {
	Message string `json:"message"`
}

func (h *ProxyHandler) handleQmsgConfig(ctx *fasthttp.RequestCtx) {
	if h.qmsgService == nil {
		writeJSON(ctx, fasthttp.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{"message": "qmsg 服务未初始化", "type": "server_error"},
		})
		return
	}
	switch string(ctx.Method()) {
	case fasthttp.MethodGet:
		writeJSON(ctx, fasthttp.StatusOK, map[string]any{
			"object": "qmsg_config",
			"config": h.qmsgService.PublicConfig(false),
		})
	case fasthttp.MethodPut:
		h.handleQmsgConfigSave(ctx)
	default:
		writeJSON(ctx, fasthttp.StatusMethodNotAllowed, map[string]any{
			"error": map[string]any{"message": "方法不允许", "type": "invalid_request_error"},
		})
	}
}

func (h *ProxyHandler) handleQmsgConfigSave(ctx *fasthttp.RequestCtx) {
	body := ctx.PostBody()
	if len(body) == 0 {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "请求体不能为空", "type": "invalid_request_error"},
		})
		return
	}
	var req qmsgConfigRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "JSON 解析失败", "type": "invalid_request_error"},
		})
		return
	}
	cfg := notify.QmsgConfig{
		Enabled:          req.Enabled,
		Key:              req.Key,
		QQ:               req.QQ,
		Bot:              req.Bot,
		TimeoutSec:       req.TimeoutSec,
		MessageTemplate:  req.MessageTemplate,
		EndpointTemplate: req.EndpointTemplate,
	}
	if strings.TrimSpace(cfg.Key) == "" {
		cfg.Key = h.qmsgService.Config().Key
	}
	if err := h.qmsgService.Save(cfg); err != nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": err.Error(), "type": "invalid_request_error"},
		})
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"object": "qmsg_config",
		"config": h.qmsgService.PublicConfig(false),
	})
}

func (h *ProxyHandler) handleQmsgTest(ctx *fasthttp.RequestCtx) {
	if h.qmsgService == nil {
		writeJSON(ctx, fasthttp.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{"message": "qmsg 服务未初始化", "type": "server_error"},
		})
		return
	}
	var req qmsgTestRequest
	if len(ctx.PostBody()) > 0 {
		if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
			writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
				"error": map[string]any{"message": "JSON 解析失败", "type": "invalid_request_error"},
			})
			return
		}
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		message = fmt.Sprintf("qmsg 测试推送\n时间：%s", time.Now().Local().Format("2006-01-02 15:04:05 MST"))
	}
	result, err := h.qmsgService.Send(context.Background(), message)
	if err != nil {
		status := fasthttp.StatusBadGateway
		errorType := "qmsg_error"
		if errors.Is(err, notify.ErrQmsgDisabled) {
			status = fasthttp.StatusBadRequest
			errorType = "invalid_request_error"
		}
		writeJSON(ctx, status, map[string]any{
			"object":  "qmsg_test_result",
			"success": false,
			"result":  result,
			"error":   map[string]any{"message": err.Error(), "type": errorType},
		})
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"object":  "qmsg_test_result",
		"success": true,
		"result":  result,
	})
}

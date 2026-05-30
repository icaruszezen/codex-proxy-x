package handler

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"codex-proxy/internal/upstream"

	"github.com/valyala/fasthttp"
)

type upstreamProviderConfigRequest struct {
	AutoSwitch bool     `json:"auto_switch"`
	BaseURL    string   `json:"base_url"`
	APIKey     string   `json:"api_key"`
	Models     []string `json:"models"`
	TimeoutSec int      `json:"timeout_sec"`
}

func (h *ProxyHandler) handleUpstreamProviderConfig(ctx *fasthttp.RequestCtx) {
	if h.upstreamProviderService == nil {
		writeJSON(ctx, fasthttp.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{"message": "上游提供商服务未初始化", "type": "server_error"},
		})
		return
	}
	switch string(ctx.Method()) {
	case fasthttp.MethodGet:
		writeJSON(ctx, fasthttp.StatusOK, map[string]any{
			"object": "upstream_provider_config",
			"config": h.upstreamProviderService.PublicConfig(),
		})
	case fasthttp.MethodPut:
		h.handleUpstreamProviderConfigSave(ctx)
	default:
		writeJSON(ctx, fasthttp.StatusMethodNotAllowed, map[string]any{
			"error": map[string]any{"message": "方法不允许", "type": "invalid_request_error"},
		})
	}
}

func (h *ProxyHandler) handleUpstreamProviderConfigSave(ctx *fasthttp.RequestCtx) {
	body := ctx.PostBody()
	if len(body) == 0 {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "请求体不能为空", "type": "invalid_request_error"},
		})
		return
	}
	var req upstreamProviderConfigRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "JSON 解析失败", "type": "invalid_request_error"},
		})
		return
	}
	cfg := upstream.ProviderConfig{
		AutoSwitch: req.AutoSwitch,
		BaseURL:    req.BaseURL,
		APIKey:     req.APIKey,
		Models:     req.Models,
		TimeoutSec: req.TimeoutSec,
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		cfg.APIKey = h.upstreamProviderService.Config().APIKey
	}
	if err := h.upstreamProviderService.Save(cfg); err != nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": err.Error(), "type": "invalid_request_error"},
		})
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"object": "upstream_provider_config",
		"config": h.upstreamProviderService.PublicConfig(),
	})
}

func (h *ProxyHandler) handleUpstreamProviderModelsFetch(ctx *fasthttp.RequestCtx) {
	if h.upstreamProviderService == nil {
		writeJSON(ctx, fasthttp.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{"message": "上游提供商服务未初始化", "type": "server_error"},
		})
		return
	}
	cfg, ok := h.providerConfigFromBodyOrSaved(ctx)
	if !ok {
		return
	}
	models, result, err := h.upstreamProviderService.FetchModels(context.Background(), cfg)
	if err != nil {
		status := fasthttp.StatusBadGateway
		if errors.Is(err, upstream.ErrProviderConfigIncomplete) {
			status = fasthttp.StatusBadRequest
		}
		writeJSON(ctx, status, map[string]any{
			"object":  "upstream_provider_models",
			"success": false,
			"result":  result,
			"error":   map[string]any{"message": err.Error(), "type": "upstream_provider_error"},
		})
		return
	}
	savedCfg := h.upstreamProviderService.Config()
	if upstream.SameCredentialTarget(cfg, savedCfg) {
		savedCfg.Models = models
		_ = h.upstreamProviderService.Save(savedCfg)
	}
	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"object":  "upstream_provider_models",
		"success": true,
		"models":  models,
		"result":  result,
		"config":  h.upstreamProviderService.PublicConfig(),
	})
}

func (h *ProxyHandler) handleUpstreamProviderTest(ctx *fasthttp.RequestCtx) {
	if h.upstreamProviderService == nil {
		writeJSON(ctx, fasthttp.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{"message": "上游提供商服务未初始化", "type": "server_error"},
		})
		return
	}
	cfg, ok := h.providerConfigFromBodyOrSaved(ctx)
	if !ok {
		return
	}
	result, err := h.upstreamProviderService.Test(context.Background(), cfg)
	if err != nil {
		status := fasthttp.StatusBadGateway
		if errors.Is(err, upstream.ErrProviderConfigIncomplete) {
			status = fasthttp.StatusBadRequest
		}
		writeJSON(ctx, status, map[string]any{
			"object":  "upstream_provider_test_result",
			"success": false,
			"result":  result,
			"error":   map[string]any{"message": err.Error(), "type": "upstream_provider_error"},
		})
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"object":  "upstream_provider_test_result",
		"success": true,
		"result":  result,
	})
}

func (h *ProxyHandler) providerConfigFromBodyOrSaved(ctx *fasthttp.RequestCtx) (upstream.ProviderConfig, bool) {
	cfg := h.upstreamProviderService.Config()
	if len(ctx.PostBody()) == 0 {
		if err := upstream.ValidateConfig(cfg); err != nil {
			writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
				"error": map[string]any{"message": err.Error(), "type": "invalid_request_error"},
			})
			return cfg, false
		}
		return cfg, true
	}
	var req upstreamProviderConfigRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "JSON 解析失败", "type": "invalid_request_error"},
		})
		return cfg, false
	}
	cfg = upstream.ProviderConfig{
		AutoSwitch: req.AutoSwitch,
		BaseURL:    req.BaseURL,
		APIKey:     req.APIKey,
		Models:     req.Models,
		TimeoutSec: req.TimeoutSec,
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		cfg.APIKey = h.upstreamProviderService.Config().APIKey
	}
	if err := upstream.ValidateConfig(cfg); err != nil {
		writeJSON(ctx, fasthttp.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": err.Error(), "type": "invalid_request_error"},
		})
		return cfg, false
	}
	return cfg, true
}

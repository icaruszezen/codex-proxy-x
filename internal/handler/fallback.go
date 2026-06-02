package handler

import (
	"context"
	"errors"
	"io"

	"codex-proxy/internal/executor"

	log "github.com/sirupsen/logrus"
)

type requestExecutorMode int

const (
	executorModeChat requestExecutorMode = iota
	executorModeResponses
	executorModeClaude
)

func (h *ProxyHandler) executeChatNonStreamWithFallback(ctx context.Context, body []byte, model string) ([]byte, error) {
	result, err := h.executor.ExecuteNonStream(ctx, h.buildRetryConfig(), body, model)
	if err == nil {
		return result, nil
	}
	if !h.shouldTryStandbyAfterPrimary(err) {
		return nil, err
	}
	lastErr := err
	if h.executor.HasProviderFallback() {
		recordFallbackToProvider(ctx, err)
		result, providerErr := h.executor.ExecuteProviderNonStream(ctx, body, model)
		if providerErr == nil {
			return result, nil
		}
		log.Warnf("上游提供商请求失败: %v", providerErr)
		lastErr = providerErr
	}
	if !h.hasStandbyFallback() {
		return nil, lastErr
	}
	recordFallbackToStandby(ctx, lastErr)
	result, standbyErr := h.executor.ExecuteNonStream(ctx, h.buildStandbyRetryConfig(), body, model)
	if standbyErr == nil {
		return result, nil
	}
	return nil, standbyErr
}

func (h *ProxyHandler) executeResponsesNonStreamWithFallback(ctx context.Context, body []byte, model string) ([]byte, error) {
	result, err := h.executor.ExecuteResponsesNonStream(ctx, h.buildRetryConfig(), body, model)
	if err == nil {
		return result, nil
	}
	if !h.shouldTryStandbyAfterPrimary(err) {
		return nil, err
	}
	lastErr := err
	if h.executor.HasProviderFallback() {
		recordFallbackToProvider(ctx, err)
		result, providerErr := h.executor.ExecuteProviderResponsesNonStream(ctx, body, model)
		if providerErr == nil {
			return result, nil
		}
		log.Warnf("上游提供商 Responses 请求失败: %v", providerErr)
		lastErr = providerErr
	}
	if !h.hasStandbyFallback() {
		return nil, lastErr
	}
	recordFallbackToStandby(ctx, lastErr)
	result, standbyErr := h.executor.ExecuteResponsesNonStream(ctx, h.buildStandbyRetryConfig(), body, model)
	if standbyErr == nil {
		return result, nil
	}
	return nil, standbyErr
}

func (h *ProxyHandler) runChatStreamWithFallback(ctx context.Context, body []byte, model string, w io.Writer, flush func()) error {
	return h.runCodexStreamWithFallback(ctx, body, model, w, flush, executorModeChat)
}

func (h *ProxyHandler) runResponsesStreamWithFallback(ctx context.Context, body []byte, model string, w io.Writer, flush func()) error {
	return h.runCodexStreamWithFallback(ctx, body, model, w, flush, executorModeResponses)
}

func (h *ProxyHandler) runClaudeStreamWithFallback(ctx context.Context, body []byte, model string, w io.Writer, flush func()) error {
	return h.runCodexStreamWithFallback(ctx, body, model, w, flush, executorModeClaude)
}

func (h *ProxyHandler) runCodexStreamWithFallback(ctx context.Context, body []byte, model string, w io.Writer, flush func(), mode requestExecutorMode) error {
	bridges := executor.CodexStreamOpenBridgeMax(h.maxRetry)
	pump := h.streamPumpForMode(mode, model)
	primaryWriter := &fallbackCountingWriter{w: w}
	err := h.executor.RunCodexStreamWithOpenBridges(ctx, h.buildRetryConfig(), body, model, primaryWriter, flush, bridges, pump)
	if err == nil {
		return nil
	}
	if !h.shouldTryStandbyAfterPrimary(err) || primaryWriter.written > 0 {
		return err
	}
	lastErr := err
	if h.executor.HasProviderFallback() {
		recordFallbackToProvider(ctx, err)
		providerWriter := &fallbackCountingWriter{w: w}
		providerErr := h.executor.RunProviderStream(ctx, body, model, providerWriter, flush, pump)
		if providerErr == nil {
			return nil
		}
		if providerWriter.written > 0 {
			return providerErr
		}
		log.Warnf("上游提供商流式请求失败: %v", providerErr)
		lastErr = providerErr
	}
	if !h.hasStandbyFallback() {
		return lastErr
	}
	recordFallbackToStandby(ctx, lastErr)
	standbyWriter := &fallbackCountingWriter{w: w}
	standbyErr := h.executor.RunCodexStreamWithOpenBridges(ctx, h.buildStandbyRetryConfig(), body, model, standbyWriter, flush, bridges, pump)
	if standbyErr == nil {
		return nil
	}
	return standbyErr
}

type fallbackCountingWriter struct {
	w       io.Writer
	written int64
}

func (w *fallbackCountingWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	if n > 0 {
		w.written += int64(n)
	}
	return n, err
}

func (h *ProxyHandler) streamPumpForMode(mode requestExecutorMode, model string) executor.CodexStreamPump {
	switch mode {
	case executorModeResponses:
		return func(s *executor.CodexResponsesStream, w io.Writer, fl func()) error {
			return s.PumpRawSSE(w, fl)
		}
	case executorModeClaude:
		return func(s *executor.CodexResponsesStream, w io.Writer, fl func()) error {
			return pumpClaudeCodexSSE(s, w, fl, model, h.debugUpstreamStream)
		}
	default:
		return func(s *executor.CodexResponsesStream, w io.Writer, fl func()) error {
			return s.PumpChatCompletion(w, fl)
		}
	}
}

func (h *ProxyHandler) hasStandbyFallback() bool {
	if h == nil || h.standbyCtrl == nil {
		return false
	}
	standby := h.standbyCtrl.Standby()
	return standby != nil && standby.AccountCount() > 0
}

func (h *ProxyHandler) shouldTryStandbyAfterPrimary(err error) bool {
	return h.shouldTryProviderAfterPrimary(err)
}

func (h *ProxyHandler) shouldTryProviderAfterPrimary(err error) bool {
	if err == nil || h == nil || h.executor == nil {
		return false
	}
	if errors.Is(err, executor.ErrProviderUnavailable) {
		return false
	}
	if statusErr, ok := err.(*executor.StatusError); ok {
		return executor.ShouldSwitchAccountForUpstreamError(statusErr.Code, statusErr.Body)
	}
	return true
}

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
	if !h.shouldTryProviderAfterPrimary(err) {
		return nil, err
	}
	if !h.executor.HasProviderFallback() {
		return nil, err
	}
	result, providerErr := h.executor.ExecuteProviderNonStream(ctx, body, model)
	if providerErr == nil {
		return result, nil
	}
	log.Warnf("上游提供商请求失败: %v", providerErr)
	return nil, providerErr
}

func (h *ProxyHandler) executeResponsesNonStreamWithFallback(ctx context.Context, body []byte, model string) ([]byte, error) {
	result, err := h.executor.ExecuteResponsesNonStream(ctx, h.buildRetryConfig(), body, model)
	if err == nil {
		return result, nil
	}
	if !h.shouldTryProviderAfterPrimary(err) {
		return nil, err
	}
	if !h.executor.HasProviderFallback() {
		return nil, err
	}
	result, providerErr := h.executor.ExecuteProviderResponsesNonStream(ctx, body, model)
	if providerErr == nil {
		return result, nil
	}
	log.Warnf("上游提供商 Responses 请求失败: %v", providerErr)
	return nil, providerErr
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
	if err == nil || !h.shouldTryProviderAfterPrimary(err) {
		return err
	}
	if primaryWriter.written > 0 {
		return err
	}
	if !h.executor.HasProviderFallback() {
		return err
	}
	providerWriter := &fallbackCountingWriter{w: w}
	providerErr := h.executor.RunProviderStream(ctx, body, model, providerWriter, flush, pump)
	if providerErr == nil {
		return nil
	}
	log.Warnf("上游提供商流式请求失败: %v", providerErr)
	return providerErr
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

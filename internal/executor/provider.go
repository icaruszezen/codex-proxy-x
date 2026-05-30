package executor

import (
	"context"
	"fmt"
	"io"
	"time"

	"codex-proxy/internal/translator"

	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

func (e *Executor) HasProviderFallback() bool {
	return e != nil && e.providerService != nil && e.providerService.AutoSwitchReady()
}

func (e *Executor) MarkProviderPrimaryAvailable() {
	if e == nil || e.providerService == nil {
		return
	}
	e.providerService.MarkPrimaryAvailable()
}

func (e *Executor) ExecuteProviderNonStream(ctx context.Context, requestBody []byte, model string) ([]byte, error) {
	startTotal := time.Now()
	reverseToolMap := translator.BuildReverseToolNameMap(requestBody)
	collected, err := e.CollectProviderResponsesSSE(ctx, requestBody, model)
	if err != nil {
		return nil, err
	}
	data := collected.Data

	if completedEvent, ok := NormalizeCompletedEventFromSSE(data); ok {
		resStr, hasOutput := translator.ConvertNonStreamResponse(completedEvent, reverseToolMap)
		if (!hasOutput || resStr == "") && len(data) > 0 {
			resStr, hasOutput = translator.ConvertStreamSSEToNonStreamResponse(data, collected.BaseModel, reverseToolMap)
		}
		if hasOutput && resStr != "" {
			log.Infof("req summary provider-nonstream model=%s convert=%v upstream_open_first_read=%v collect=%v total=%v lines=%d", collected.BaseModel, collected.ConvertDur, collected.SendDur, collected.CollectDur, time.Since(startTotal), collected.Lines)
			return []byte(resStr), nil
		}
	}

	log.Infof("req summary provider-nonstream (empty after stream aggregation) model=%s total=%v lines=%d", collected.BaseModel, time.Since(startTotal), collected.Lines)
	return nil, ErrEmptyResponse
}

func (e *Executor) ExecuteProviderResponsesNonStream(ctx context.Context, requestBody []byte, model string) ([]byte, error) {
	startTotal := time.Now()
	collected, err := e.CollectProviderResponsesSSE(ctx, requestBody, model)
	if err != nil {
		return nil, err
	}
	data := collected.Data

	if completedEvent, ok := NormalizeCompletedEventFromSSE(data); ok {
		respRaw := gjson.GetBytes(completedEvent, "response")
		if !respRaw.Exists() {
			return nil, fmt.Errorf("未收到 response.completed.response 对象")
		}
		resp := []byte(respRaw.Raw)
		log.Infof("req summary provider-responses-nonstream model=%s convert=%v upstream_open_first_read=%v collect=%v total=%v lines=%d", collected.BaseModel, collected.ConvertDur, collected.SendDur, collected.CollectDur, time.Since(startTotal), collected.Lines)
		return resp, nil
	}

	log.Infof("req summary provider-responses-nonstream model=%s total=%v (no completed)", collected.BaseModel, time.Since(startTotal))
	return nil, fmt.Errorf("未收到 response.completed 事件")
}

func (e *Executor) RunProviderStream(ctx context.Context, requestBody []byte, model string, w io.Writer, flush func(), pump CodexStreamPump) error {
	s, err := e.OpenProviderResponsesStream(ctx, requestBody, model)
	if err != nil {
		return err
	}
	return pump(s, w, flush)
}

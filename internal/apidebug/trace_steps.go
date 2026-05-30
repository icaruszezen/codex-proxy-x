package apidebug

import (
	"context"
	"fmt"
	"net/http"
)

func TraceRequestTransform(ctx context.Context, baseModel string, useProvider bool, codexBody []byte) {
	c := FromContext(ctx)
	if c == nil {
		return
	}
	c.AddStep(StepInput{
		Name:  "request_transform",
		Phase: PhaseInfo,
		Body:  append([]byte(nil), codexBody...),
		Note:  fmt.Sprintf("base_model=%s provider=%v", baseModel, useProvider),
	})
}

func TraceCodexUpstreamRequest(ctx context.Context, attempt, max int, account, apiURL string, headers http.Header, body []byte) {
	c := FromContext(ctx)
	if c == nil {
		return
	}
	c.AddStep(StepInput{
		Name:     "codex_upstream_request",
		Phase:    PhaseRequest,
		Account:  account,
		URL:      apiURL,
		HTTPHeader: headers,
		Body:     append([]byte(nil), body...),
		Note:     fmt.Sprintf("attempt=%d/%d", attempt, max),
	})
}

func TraceCodexUpstreamResponse(ctx context.Context, attempt int, account string, statusCode int, body []byte, phase string) {
	c := FromContext(ctx)
	if c == nil {
		return
	}
	if phase == "" {
		if statusCode >= 200 && statusCode < 300 {
			phase = PhaseResponse
		} else {
			phase = PhaseError
		}
	}
	name := "codex_upstream_response"
	if phase == PhaseError {
		name = "codex_upstream_error"
	}
	note := ""
	if phase == PhaseResponse && len(body) == 0 {
		note = "stream body omitted"
	}
	c.AddStep(StepInput{
		Name:       name,
		Phase:      phase,
		Account:    account,
		StatusCode: statusCode,
		Body:       body,
		Note:       note,
	})
}

func TraceCodexUpstreamNetError(ctx context.Context, attempt int, account, apiURL string, err error) {
	c := FromContext(ctx)
	if c == nil || err == nil {
		return
	}
	c.AddStep(StepInput{
		Name:    "codex_upstream_error",
		Phase:   PhaseError,
		Account: account,
		URL:     apiURL,
		Note:    fmt.Sprintf("attempt=%d network: %v", attempt, err),
	})
}

func TraceProviderUpstreamRequest(ctx context.Context, apiURL string, body []byte) {
	c := FromContext(ctx)
	if c == nil {
		return
	}
	c.AddStep(StepInput{
		Name:  "provider_upstream_request",
		Phase: PhaseRequest,
		URL:   apiURL,
		Body:  append([]byte(nil), body...),
	})
}

func TraceProviderUpstreamResponse(ctx context.Context, statusCode int, body []byte, note string) {
	c := FromContext(ctx)
	if c == nil {
		return
	}
	phase := PhaseResponse
	name := "provider_upstream_response"
	if statusCode < 200 || statusCode >= 300 {
		phase = PhaseError
		name = "provider_upstream_error"
	}
	c.AddStep(StepInput{
		Name:       name,
		Phase:      phase,
		StatusCode: statusCode,
		Body:       body,
		Note:       note,
	})
}

func TraceProviderUpstreamError(ctx context.Context, apiURL string, err error) {
	c := FromContext(ctx)
	if c == nil || err == nil {
		return
	}
	c.AddStep(StepInput{
		Name:  "provider_upstream_error",
		Phase: PhaseError,
		URL:   apiURL,
		Note:  err.Error(),
	})
}

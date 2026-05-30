package handler

import (
	"context"
	"io"

	"codex-proxy/internal/apidebug"

	"github.com/valyala/fasthttp"
)

type apiTraceSession struct {
	collector *apidebug.Collector
	store     *apidebug.Store
	finished  bool
}

func (h *ProxyHandler) startAPITrace(ctx *fasthttp.RequestCtx, model string, stream bool, body []byte) (*apiTraceSession, context.Context) {
	if h == nil || h.apiDebug == nil || !h.apiDebug.Enabled() {
		return nil, context.Background()
	}
	collector := apidebug.NewCollector(string(ctx.Method()), string(ctx.Path()), model, stream)
	collector.AddStep(apidebug.StepInput{
		Name:  "downstream_request",
		Phase: apidebug.PhaseRequest,
		Body:  append([]byte(nil), body...),
	})
	sess := &apiTraceSession{
		collector: collector,
		store:     h.apiDebug,
	}
	return sess, apidebug.WithCollector(context.Background(), collector)
}

func (s *apiTraceSession) context(base context.Context) context.Context {
	if s == nil || s.collector == nil {
		return base
	}
	if base == nil {
		base = context.Background()
	}
	return apidebug.WithCollector(base, s.collector)
}

func (s *apiTraceSession) recordDownstreamResponse(body []byte) {
	if s == nil || s.collector == nil {
		return
	}
	s.collector.AddStep(apidebug.StepInput{
		Name:  "downstream_response",
		Phase: apidebug.PhaseResponse,
		Body:  append([]byte(nil), body...),
	})
}

func (s *apiTraceSession) finish(success bool, err error) {
	if s == nil || s.collector == nil || s.finished {
		return
	}
	s.finished = true
	rec := s.collector.Finish(success, err)
	if s.store != nil {
		s.store.Push(rec)
	}
}

func (s *apiTraceSession) wrapWriter(w io.Writer) io.Writer {
	if s == nil || s.collector == nil {
		return w
	}
	return apidebug.NewTraceWriter(w, s.collector)
}

func (s *apiTraceSession) finishStream(traceWriter *apidebug.TraceWriter, success bool, err error) {
	if traceWriter != nil {
		traceWriter.RecordDownstreamResponse()
	}
	s.finish(success, err)
}

func recordFallbackToProvider(ctx context.Context, primaryErr error) {
	collector := apidebug.FromContext(ctx)
	if collector == nil || primaryErr == nil {
		return
	}
	note := primaryErr.Error()
	collector.AddStep(apidebug.StepInput{
		Name:  "fallback_to_provider",
		Phase: apidebug.PhaseInfo,
		Note:  note,
	})
}

package apidebug

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ctxKey struct{}

type StepInput struct {
	Name       string
	Phase      string
	Account    string
	URL        string
	StatusCode int
	Headers    map[string]string
	HTTPHeader http.Header
	Body       []byte
	BodyText   string
	Note       string
}

type Collector struct {
	mu        sync.Mutex
	id        string
	startedAt time.Time
	method    string
	path      string
	model     string
	stream    bool
	steps     []Step
	hasCodex  bool
	hasProv   bool
	finished  bool
}

func NewCollector(method, path, model string, stream bool) *Collector {
	return &Collector{
		id:        newTraceID(),
		startedAt: time.Now(),
		method:    method,
		path:      path,
		model:     model,
		stream:    stream,
		steps:     make([]Step, 0, 8),
	}
}

func newTraceID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func WithCollector(ctx context.Context, c *Collector) context.Context {
	if ctx == nil || c == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, c)
}

func FromContext(ctx context.Context) *Collector {
	if ctx == nil {
		return nil
	}
	c, _ := ctx.Value(ctxKey{}).(*Collector)
	return c
}

func (c *Collector) AddStep(in StepInput) {
	if c == nil {
		return
	}
	name := strings.TrimSpace(in.Name)
	phase := strings.TrimSpace(in.Phase)
	if name == "" || phase == "" {
		return
	}
	switch name {
	case "codex_upstream_request", "codex_upstream_response", "codex_upstream_error":
		c.hasCodex = true
	case "provider_upstream_request", "provider_upstream_response", "provider_upstream_error":
		c.hasProv = true
	}

	var bodyText string
	var truncated bool
	switch {
	case in.BodyText != "":
		bodyText, truncated = TruncateString(in.BodyText)
	case len(in.Body) > 0:
		bodyText, truncated = TruncateBody(in.Body)
	}

	headers := in.Headers
	if headers == nil && in.HTTPHeader != nil {
		headers = SanitizeHeaders(in.HTTPHeader)
	} else if headers != nil {
		headers = SanitizeHeaderMap(headers)
	}

	step := Step{
		At:         time.Now(),
		Name:       name,
		Phase:      phase,
		Account:    strings.TrimSpace(in.Account),
		URL:        strings.TrimSpace(in.URL),
		StatusCode: in.StatusCode,
		Headers:    headers,
		Body:       bodyText,
		Truncated:  truncated,
		Note:       strings.TrimSpace(in.Note),
	}

	c.mu.Lock()
	c.steps = append(c.steps, step)
	c.mu.Unlock()
}

func (c *Collector) inferRoute() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.hasCodex && c.hasProv {
		return RouteCodexThenProvider
	}
	if c.hasProv {
		return RouteProvider
	}
	return RouteCodex
}

func (c *Collector) Finish(success bool, err error) Record {
	if c == nil {
		return Record{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.finished {
		return c.snapshotLocked(success, err)
	}
	c.finished = true
	return c.snapshotLocked(success, err)
}

func (c *Collector) snapshotLocked(success bool, err error) Record {
	steps := append([]Step(nil), c.steps...)
	rec := Record{
		ID:         c.id,
		StartedAt:  c.startedAt,
		DurationMs: time.Since(c.startedAt).Milliseconds(),
		Method:     c.method,
		Path:       c.path,
		Model:      c.model,
		Stream:     c.stream,
		Success:    success,
		Route:      RouteCodex,
		Steps:      steps,
	}
	if c.hasCodex && c.hasProv {
		rec.Route = RouteCodexThenProvider
	} else if c.hasProv {
		rec.Route = RouteProvider
	}
	if err != nil {
		rec.Error = err.Error()
	}
	return rec
}

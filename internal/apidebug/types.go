package apidebug

import "time"

const (
	MaxBodyBytes   = 64 * 1024
	DefaultMaxSize = 20

	PhaseRequest  = "request"
	PhaseResponse = "response"
	PhaseError    = "error"
	PhaseInfo     = "info"

	RouteCodex             = "codex"
	RouteProvider          = "provider"
	RouteCodexThenProvider = "codex_then_provider"
)

type Config struct {
	Enabled bool `json:"enabled"`
}

type Record struct {
	ID         string    `json:"id"`
	StartedAt  time.Time `json:"started_at"`
	DurationMs int64     `json:"duration_ms"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Model      string    `json:"model,omitempty"`
	Stream     bool      `json:"stream"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
	Route      string    `json:"route"`
	Steps      []Step    `json:"steps"`
}

type Step struct {
	At         time.Time         `json:"at"`
	Name       string            `json:"name"`
	Phase      string            `json:"phase"`
	Account    string            `json:"account,omitempty"`
	URL        string            `json:"url,omitempty"`
	StatusCode int               `json:"status_code,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	Truncated  bool              `json:"truncated,omitempty"`
	Note       string            `json:"note,omitempty"`
}

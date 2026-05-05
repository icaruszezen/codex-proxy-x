package auth

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultRecentAccountEventLimit = 20
	maxRecentAccountEventLimit     = 100
	accountEventLogFileName        = "account-events.jsonl"
)

type accountEventLog struct {
	path string
	mu   sync.Mutex
}

func newAccountEventLog(authDir string) *accountEventLog {
	return &accountEventLog{path: resolveAccountEventLogPath(authDir)}
}

func resolveAccountEventLogPath(authDir string) string {
	dir := strings.TrimSpace(authDir)
	if dir == "" {
		dir = filepath.Join(".", "data")
	}
	return filepath.Join(dir, accountEventLogFileName)
}

func clampRecentAccountEventLimit(limit int) int {
	if limit <= 0 {
		return defaultRecentAccountEventLimit
	}
	if limit > maxRecentAccountEventLimit {
		return maxRecentAccountEventLimit
	}
	return limit
}

func (l *accountEventLog) Append(event AccountEvent) error {
	if l == nil || l.path == "" {
		return nil
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	line, err := json.Marshal(event)
	if err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

func (l *accountEventLog) ReadRecent(limit int) ([]AccountEvent, error) {
	if l == nil || l.path == "" {
		return []AccountEvent{}, nil
	}
	limit = clampRecentAccountEventLimit(limit)

	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.Open(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []AccountEvent{}, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	events := make([]AccountEvent, 0, limit)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event AccountEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if len(events) == limit {
			copy(events, events[1:])
			events[limit-1] = event
			continue
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}
	return events, nil
}

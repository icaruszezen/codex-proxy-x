package apidebug

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

const configFileName = "api-debug-config.json"

type Store struct {
	mu      sync.RWMutex
	path    string
	enabled bool
	records []Record
	maxSize int
}

func DefaultConfigPath(authDir string) string {
	authDir = strings.TrimSpace(authDir)
	if authDir == "" {
		authDir = "."
	}
	return filepath.Join(authDir, configFileName)
}

func NewStore(configPath string) (*Store, error) {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return nil, errors.New("api debug 配置路径不能为空")
	}
	s := &Store{
		path:    configPath,
		maxSize: DefaultMaxSize,
		records: make([]Record, 0, DefaultMaxSize),
	}
	if err := s.Load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Load() error {
	if s == nil {
		return errors.New("api debug store 未初始化")
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return s.saveConfigLocked()
		}
		return err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	s.mu.Lock()
	s.enabled = cfg.Enabled
	s.mu.Unlock()
	if cfg.Enabled {
		log.Warnf("API 调试记录已开启（%s），请求内容可能包含敏感信息", s.path)
	}
	return nil
}

func (s *Store) saveConfigLocked() error {
	cfg := Config{Enabled: s.enabled}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) Enabled() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

func (s *Store) Config() Config {
	if s == nil {
		return Config{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Config{Enabled: s.enabled}
}

func (s *Store) SetEnabled(enabled bool) error {
	if s == nil {
		return errors.New("api debug store 未初始化")
	}
	s.mu.Lock()
	prev := s.enabled
	s.enabled = enabled
	if err := s.saveConfigLocked(); err != nil {
		s.enabled = prev
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()
	if enabled && !prev {
		log.Warnf("API 调试记录已开启，请求内容可能包含敏感信息")
	} else if !enabled && prev {
		log.Infof("API 调试记录已关闭")
	}
	return nil
}

func (s *Store) Push(rec Record) {
	if s == nil || rec.ID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.enabled {
		return
	}
	s.records = append([]Record{rec}, s.records...)
	if len(s.records) > s.maxSize {
		s.records = s.records[:s.maxSize]
	}
}

func (s *Store) List() []Record {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Record, len(s.records))
	copy(out, s.records)
	return out
}

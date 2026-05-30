package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultNewAPITimeoutSec = 10
	newapiConfigFileName    = "newapi-config.json"
)

var (
	ErrNewAPIAutoSwitchDisabled = errors.New("NewAPI 自动禁用/启用未开启")
	ErrNewAPIConfigIncomplete   = errors.New("NewAPI 配置不完整")
)

type NewAPIConfig struct {
	AutoSwitch  bool   `json:"auto_switch"`
	BaseURL     string `json:"base_url,omitempty"`
	AdminToken  string `json:"admin_token,omitempty"`
	AdminUserID int    `json:"admin_user_id,omitempty"`
	ChannelID   int    `json:"channel_id,omitempty"`
	TimeoutSec  int    `json:"timeout_sec,omitempty"`
}

type PublicNewAPIConfig struct {
	AutoSwitch  bool   `json:"auto_switch"`
	BaseURL     string `json:"base_url,omitempty"`
	TokenMasked string `json:"token_masked,omitempty"`
	AdminUserID int    `json:"admin_user_id,omitempty"`
	ChannelID   int    `json:"channel_id,omitempty"`
	TimeoutSec  int    `json:"timeout_sec"`
	Configured  bool   `json:"configured"`
}

type NewAPIResult struct {
	Success    bool   `json:"success"`
	StatusCode int    `json:"status_code"`
	Body       string `json:"body,omitempty"`
}

type NewAPIService struct {
	mu         sync.RWMutex
	path       string
	cfg        NewAPIConfig
	httpClient *http.Client
}

func DefaultNewAPIConfigPath(authDir string) string {
	authDir = strings.TrimSpace(authDir)
	if authDir == "" {
		authDir = "."
	}
	return filepath.Join(authDir, newapiConfigFileName)
}

func NewNewAPIService(configPath string) (*NewAPIService, error) {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return nil, errors.New("NewAPI 配置路径不能为空")
	}
	s := &NewAPIService{
		path:       configPath,
		cfg:        normalizeNewAPIConfig(NewAPIConfig{}),
		httpClient: &http.Client{},
	}
	if err := s.Load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *NewAPIService) Load() error {
	if s == nil {
		return errors.New("NewAPI 服务未初始化")
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.mu.Lock()
			s.cfg = normalizeNewAPIConfig(NewAPIConfig{})
			s.mu.Unlock()
			return nil
		}
		return fmt.Errorf("读取 NewAPI 配置失败: %w", err)
	}
	var cfg NewAPIConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("解析 NewAPI 配置失败: %w", err)
	}
	s.mu.Lock()
	s.cfg = normalizeNewAPIConfig(cfg)
	s.mu.Unlock()
	return nil
}

func (s *NewAPIService) Save(cfg NewAPIConfig) error {
	if s == nil {
		return errors.New("NewAPI 服务未初始化")
	}
	cfg = normalizeNewAPIConfig(cfg)
	if cfg.AutoSwitch {
		if err := validateNewAPIConfig(cfg); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("创建 NewAPI 配置目录失败: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 NewAPI 配置失败: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("写入 NewAPI 临时配置失败: %w", err)
	}
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmp)
		return fmt.Errorf("替换 NewAPI 配置失败: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("保存 NewAPI 配置失败: %w", err)
	}
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	return nil
}

func (s *NewAPIService) Config() NewAPIConfig {
	if s == nil {
		return normalizeNewAPIConfig(NewAPIConfig{})
	}
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	return normalizeNewAPIConfig(cfg)
}

func (s *NewAPIService) PublicConfig() PublicNewAPIConfig {
	cfg := s.Config()
	return PublicNewAPIConfig{
		AutoSwitch:  cfg.AutoSwitch,
		BaseURL:     cfg.BaseURL,
		TokenMasked: maskNewAPIToken(cfg.AdminToken),
		AdminUserID: cfg.AdminUserID,
		ChannelID:   cfg.ChannelID,
		TimeoutSec:  cfg.TimeoutSec,
		Configured:  isNewAPIConfigured(cfg),
	}
}

func (s *NewAPIService) EnableChannel(ctx context.Context) (*NewAPIResult, error) {
	return s.setChannelStatus(ctx, 1, true)
}

func (s *NewAPIService) DisableChannel(ctx context.Context) (*NewAPIResult, error) {
	return s.setChannelStatus(ctx, 2, true)
}

func (s *NewAPIService) TestEnableChannel(ctx context.Context) (*NewAPIResult, error) {
	return s.setChannelStatus(ctx, 1, false)
}

func (s *NewAPIService) TestDisableChannel(ctx context.Context) (*NewAPIResult, error) {
	return s.setChannelStatus(ctx, 2, false)
}

func (s *NewAPIService) setChannelStatus(ctx context.Context, status int, requireAutoSwitch bool) (*NewAPIResult, error) {
	if s == nil {
		return nil, errors.New("NewAPI 服务未初始化")
	}
	if status != 1 && status != 2 {
		return nil, errors.New("NewAPI 渠道状态只能为 1 或 2")
	}
	cfg := s.Config()
	if requireAutoSwitch && !cfg.AutoSwitch {
		return nil, ErrNewAPIAutoSwitchDisabled
	}
	if err := validateNewAPIConfig(cfg); err != nil {
		return nil, err
	}
	endpoint, err := newAPIChannelEndpoint(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	payload := map[string]int{
		"id":     cfg.ChannelID,
		"status": status,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化 NewAPI 请求失败: %w", err)
	}
	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = DefaultNewAPITimeoutSec * time.Second
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建 NewAPI 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.AdminToken)
	req.Header.Set("New-Api-User", strconv.Itoa(cfg.AdminUserID))

	res, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用 NewAPI 失败: %w", err)
	}
	defer res.Body.Close()
	resBody, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取 NewAPI 响应失败: %w", err)
	}
	result := &NewAPIResult{
		Success:    res.StatusCode >= 200 && res.StatusCode < 300,
		StatusCode: res.StatusCode,
		Body:       strings.TrimSpace(string(resBody)),
	}
	if !result.Success {
		return result, fmt.Errorf("NewAPI 返回 HTTP %d: %s", res.StatusCode, result.Body)
	}
	return result, nil
}

func normalizeNewAPIConfig(cfg NewAPIConfig) NewAPIConfig {
	cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)
	cfg.AdminToken = strings.TrimSpace(cfg.AdminToken)
	if cfg.TimeoutSec <= 0 {
		cfg.TimeoutSec = DefaultNewAPITimeoutSec
	}
	if cfg.TimeoutSec > 60 {
		cfg.TimeoutSec = 60
	}
	return cfg
}

func validateNewAPIConfig(cfg NewAPIConfig) error {
	cfg = normalizeNewAPIConfig(cfg)
	if cfg.BaseURL == "" || cfg.AdminToken == "" || cfg.AdminUserID <= 0 || cfg.ChannelID <= 0 {
		return ErrNewAPIConfigIncomplete
	}
	if _, err := newAPIChannelEndpoint(cfg.BaseURL); err != nil {
		return err
	}
	return nil
}

func isNewAPIConfigured(cfg NewAPIConfig) bool {
	return validateNewAPIConfig(cfg) == nil
}

func newAPIChannelEndpoint(baseURL string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("NewAPI 项目地址无效")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("NewAPI 项目地址仅支持 http/https")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api/channel/"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func maskNewAPIToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return strings.Repeat("*", len(token))
	}
	return token[:4] + strings.Repeat("*", len(token)-8) + token[len(token)-4:]
}

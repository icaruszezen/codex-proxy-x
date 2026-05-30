package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codex-proxy/internal/netutil"
	"codex-proxy/internal/notify"

	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

const (
	DefaultProviderTimeoutSec = 15
	providerConfigFileName    = "upstream-provider-config.json"
)

var (
	ErrProviderAutoSwitchDisabled = errors.New("上游提供商自动切换未开启")
	ErrProviderConfigIncomplete   = errors.New("上游提供商配置不完整")
)

type ProviderConfig struct {
	AutoSwitch bool     `json:"auto_switch"`
	BaseURL    string   `json:"base_url,omitempty"`
	APIKey     string   `json:"api_key,omitempty"`
	Models     []string `json:"models,omitempty"`
	TimeoutSec int      `json:"timeout_sec,omitempty"`
}

type PublicProviderConfig struct {
	AutoSwitch   bool     `json:"auto_switch"`
	BaseURL      string   `json:"base_url,omitempty"`
	APIKeyMasked string   `json:"api_key_masked,omitempty"`
	Models       []string `json:"models,omitempty"`
	TimeoutSec   int      `json:"timeout_sec"`
	Configured   bool     `json:"configured"`
	Active       bool     `json:"active"`
}

type ProviderResult struct {
	Success    bool   `json:"success"`
	StatusCode int    `json:"status_code"`
	Body       string `json:"body,omitempty"`
	ModelCount int    `json:"model_count,omitempty"`
}

type Service struct {
	mu          sync.RWMutex
	path        string
	cfg         ProviderConfig
	httpClient  *http.Client
	qmsgService *notify.Service
	active      atomic.Bool
}

func DefaultConfigPath(authDir string) string {
	authDir = strings.TrimSpace(authDir)
	if authDir == "" {
		authDir = "."
	}
	return filepath.Join(authDir, providerConfigFileName)
}

func NewService(configPath, proxyURL string) (*Service, error) {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return nil, errors.New("上游提供商配置路径不能为空")
	}
	s := &Service{
		path:       configPath,
		cfg:        normalizeConfig(ProviderConfig{}),
		httpClient: newProviderHTTPClient(proxyURL),
	}
	if err := s.Load(); err != nil {
		return nil, err
	}
	return s, nil
}

func newProviderHTTPClient(proxyURL string) *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 60 * time.Second,
	}
	dialCtx := netutil.BuildUpstreamDialContext(dialer, proxyURL, "", "")
	dialCtx = netutil.WrapDialWithTCPNoDelay(dialCtx)
	transport := netutil.NewUpstreamTransport(netutil.UpstreamTransportConfig{
		DialContext:         dialCtx,
		ProxyURL:            proxyURL,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		EnableHTTP2:         true,
		IdleConnTimeout:     120 * time.Second,
		WriteBufferSize:     32 * 1024,
		ReadBufferSize:      32 * 1024,
		DisableCompression:  true,
	})
	return &http.Client{Transport: transport, Timeout: 0}
}

func (s *Service) SetQmsgService(service *notify.Service) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.qmsgService = service
	s.mu.Unlock()
}

func (s *Service) Load() error {
	if s == nil {
		return errors.New("上游提供商服务未初始化")
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.mu.Lock()
			s.cfg = normalizeConfig(ProviderConfig{})
			s.mu.Unlock()
			return nil
		}
		return fmt.Errorf("读取上游提供商配置失败: %w", err)
	}
	var cfg ProviderConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("解析上游提供商配置失败: %w", err)
	}
	s.mu.Lock()
	s.cfg = normalizeConfig(cfg)
	s.mu.Unlock()
	return nil
}

func (s *Service) Save(cfg ProviderConfig) error {
	if s == nil {
		return errors.New("上游提供商服务未初始化")
	}
	cfg = normalizeConfig(cfg)
	if cfg.AutoSwitch {
		if err := ValidateConfig(cfg); err != nil {
			return err
		}
	} else if strings.TrimSpace(cfg.BaseURL) != "" {
		if _, err := NormalizeBaseURL(cfg.BaseURL); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("创建上游提供商配置目录失败: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化上游提供商配置失败: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("写入上游提供商临时配置失败: %w", err)
	}
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmp)
		return fmt.Errorf("替换上游提供商配置失败: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("保存上游提供商配置失败: %w", err)
	}
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	if !cfg.AutoSwitch {
		s.active.Store(false)
	}
	return nil
}

func (s *Service) Config() ProviderConfig {
	if s == nil {
		return normalizeConfig(ProviderConfig{})
	}
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	return normalizeConfig(cfg)
}

func (s *Service) PublicConfig() PublicProviderConfig {
	cfg := s.Config()
	return PublicProviderConfig{
		AutoSwitch:   cfg.AutoSwitch,
		BaseURL:      cfg.BaseURL,
		APIKeyMasked: maskAPIKey(cfg.APIKey),
		Models:       append([]string(nil), cfg.Models...),
		TimeoutSec:   cfg.TimeoutSec,
		Configured:   IsConfigured(cfg),
		Active:       s != nil && s.active.Load(),
	}
}

func (s *Service) AutoSwitchReady() bool {
	if s == nil {
		return false
	}
	cfg := s.Config()
	return cfg.AutoSwitch && ValidateConfig(cfg) == nil
}

func (s *Service) MarkPrimaryAvailable() {
	if s == nil {
		return
	}
	if !s.active.CompareAndSwap(true, false) {
		return
	}
	log.Infof("上游提供商已停用: 主账号池已恢复")
	s.sendQmsg("[上游提供商] 主账号池已恢复，已停止使用上游 API 提供商")
}

func (s *Service) markProviderActive(model string, cfg ProviderConfig) {
	if s == nil {
		return
	}
	if !s.active.CompareAndSwap(false, true) {
		return
	}
	log.Warnf("上游提供商已启用: 主账号池无可用账号，已切换至 %s model=%s", cfg.BaseURL, model)
	s.sendQmsg("[上游提供商] 主账号池已无可用账号，已自动切换到上游 API 提供商\n地址：%s\n模型：%s", cfg.BaseURL, model)
}

func (s *Service) sendQmsg(format string, args ...any) {
	if s == nil {
		return
	}
	s.mu.RLock()
	qmsgService := s.qmsgService
	s.mu.RUnlock()
	if qmsgService == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := qmsgService.Send(ctx, msg); err != nil && err != notify.ErrQmsgDisabled {
			log.Warnf("上游提供商 qmsg 通知发送失败: %v", err)
		}
	}()
}

func (s *Service) SendResponses(ctx context.Context, body []byte, stream bool, model string) (*http.Response, error) {
	if s == nil {
		return nil, errors.New("上游提供商服务未初始化")
	}
	cfg := s.Config()
	if !cfg.AutoSwitch {
		return nil, ErrProviderAutoSwitchDisabled
	}
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}
	endpoint, err := ResponsesEndpoint(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	requestCtx, cancel := context.WithCancel(ctx)
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("创建上游提供商请求失败: %w", err)
	}
	applyProviderHeaders(req, cfg.APIKey, stream)
	resp, err := s.doWithResponseHeaderTimeout(requestCtx, cancel, req, cfg.TimeoutSec)
	if err != nil {
		return nil, fmt.Errorf("调用上游提供商失败: %w", err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		s.markProviderActive(model, cfg)
		return resp, nil
	}
	errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
	return nil, fmt.Errorf("上游提供商返回 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
}

type providerDoResult struct {
	resp *http.Response
	err  error
}

type cancelOnCloseReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
	once   sync.Once
}

func (c *cancelOnCloseReadCloser) Close() error {
	err := c.ReadCloser.Close()
	c.once.Do(c.cancel)
	return err
}

func (s *Service) doWithResponseHeaderTimeout(ctx context.Context, cancel context.CancelFunc, req *http.Request, timeoutSec int) (*http.Response, error) {
	if timeoutSec <= 0 {
		timeoutSec = DefaultProviderTimeoutSec
	}
	done := make(chan providerDoResult, 1)
	go func() {
		resp, err := s.httpClient.Do(req)
		done <- providerDoResult{resp: resp, err: err}
	}()

	timer := time.NewTimer(time.Duration(timeoutSec) * time.Second)
	defer timer.Stop()
	select {
	case result := <-done:
		if result.err != nil {
			cancel()
			return nil, result.err
		}
		if result.resp != nil && result.resp.Body != nil {
			result.resp.Body = &cancelOnCloseReadCloser{ReadCloser: result.resp.Body, cancel: cancel}
		} else {
			cancel()
		}
		return result.resp, nil
	case <-timer.C:
		cancel()
		go closeProviderDoResult(done)
		return nil, fmt.Errorf("等待响应头超过 %d 秒", timeoutSec)
	case <-ctx.Done():
		cancel()
		go closeProviderDoResult(done)
		return nil, ctx.Err()
	}
}

func closeProviderDoResult(done <-chan providerDoResult) {
	result := <-done
	if result.resp != nil && result.resp.Body != nil {
		_ = result.resp.Body.Close()
	}
}

func (s *Service) FetchModels(ctx context.Context, cfg ProviderConfig) ([]string, *ProviderResult, error) {
	cfg = normalizeConfig(cfg)
	if err := ValidateConfig(cfg); err != nil {
		return nil, nil, err
	}
	endpoint, err := ModelsEndpoint(cfg.BaseURL)
	if err != nil {
		return nil, nil, err
	}
	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = DefaultProviderTimeoutSec * time.Second
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("创建模型列表请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("调用模型列表接口失败: %w", err)
	}
	defer resp.Body.Close()
	resBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, nil, fmt.Errorf("读取模型列表响应失败: %w", err)
	}
	result := &ProviderResult{
		Success:    resp.StatusCode >= 200 && resp.StatusCode < 300,
		StatusCode: resp.StatusCode,
		Body:       strings.TrimSpace(string(resBody)),
	}
	if !result.Success {
		return nil, result, fmt.Errorf("模型列表接口返回 HTTP %d: %s", resp.StatusCode, result.Body)
	}
	models := parseModelIDs(resBody)
	result.ModelCount = len(models)
	return models, result, nil
}

func (s *Service) Test(ctx context.Context, cfg ProviderConfig) (*ProviderResult, error) {
	models, result, err := s.FetchModels(ctx, cfg)
	if result != nil {
		result.ModelCount = len(models)
	}
	return result, err
}

func applyProviderHeaders(req *http.Request, apiKey string, stream bool) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}
}

func parseModelIDs(body []byte) []string {
	seen := make(map[string]bool)
	var ids []string
	data := gjson.GetBytes(body, "data")
	if data.IsArray() {
		for _, item := range data.Array() {
			id := strings.TrimSpace(item.Get("id").String())
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		root := gjson.ParseBytes(body)
		if root.IsArray() {
			for _, item := range root.Array() {
				id := strings.TrimSpace(item.Get("id").String())
				if id == "" {
					id = strings.TrimSpace(item.String())
				}
				if id == "" || seen[id] {
					continue
				}
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	sort.Strings(ids)
	return ids
}

func normalizeConfig(cfg ProviderConfig) ProviderConfig {
	cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)
	if cfg.BaseURL != "" {
		if baseURL, err := NormalizeBaseURL(cfg.BaseURL); err == nil {
			cfg.BaseURL = baseURL
		}
	}
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.Models = normalizeModels(cfg.Models)
	if cfg.TimeoutSec <= 0 {
		cfg.TimeoutSec = DefaultProviderTimeoutSec
	}
	if cfg.TimeoutSec > 120 {
		cfg.TimeoutSec = 120
	}
	return cfg
}

func normalizeModels(models []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" || seen[model] {
			continue
		}
		seen[model] = true
		out = append(out, model)
	}
	sort.Strings(out)
	return out
}

func ValidateConfig(cfg ProviderConfig) error {
	cfg = normalizeConfig(cfg)
	if cfg.BaseURL == "" || cfg.APIKey == "" {
		return ErrProviderConfigIncomplete
	}
	if _, err := NormalizeBaseURL(cfg.BaseURL); err != nil {
		return err
	}
	return nil
}

func IsConfigured(cfg ProviderConfig) bool {
	return ValidateConfig(cfg) == nil
}

func SameCredentialTarget(a, b ProviderConfig) bool {
	a = normalizeConfig(a)
	b = normalizeConfig(b)
	return a.BaseURL != "" && a.BaseURL == b.BaseURL && a.APIKey != "" && a.APIKey == b.APIKey
}

func NormalizeBaseURL(baseURL string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "", ErrProviderConfigIncomplete
	}
	if !strings.Contains(baseURL, "://") {
		baseURL = "https://" + baseURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("上游提供商 API 地址无效")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("上游提供商 API 地址仅支持 http/https")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	cleanPath := strings.TrimRight(parsed.Path, "/")
	if cleanPath == "/" {
		cleanPath = ""
	}
	last := strings.ToLower(path.Base(cleanPath))
	if last == "responses" || last == "models" || last == "chat" || last == "completions" {
		cleanPath = path.Dir(cleanPath)
		if cleanPath == "." || cleanPath == "/" {
			cleanPath = ""
		}
	}
	if !strings.HasSuffix(strings.ToLower(cleanPath), "/v1") && strings.ToLower(cleanPath) != "/v1" {
		cleanPath = strings.TrimRight(cleanPath, "/") + "/v1"
	}
	parsed.Path = cleanPath
	return strings.TrimRight(parsed.String(), "/"), nil
}

func ResponsesEndpoint(baseURL string) (string, error) {
	base, err := NormalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(base, "/") + "/responses", nil
}

func ModelsEndpoint(baseURL string) (string, error) {
	base, err := NormalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(base, "/") + "/models", nil
}

func maskAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

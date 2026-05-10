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
	"strings"
	"sync"
	"time"
)

const (
	DefaultQmsgEndpointTemplate = "https://qmsg.zendee.cn/jsend/{key}"
	DefaultQmsgTimeoutSec       = 10
	DefaultQmsgMessageTemplate  = "账号自动{{action}}通知\n邮箱：{{email}}\n原因：{{reason_code}}\n详情：{{detail}}\n存储：{{storage_mode}}\n时间：{{timestamp}}"
	qmsgConfigFileName          = "qmsg-config.json"
)

var ErrQmsgDisabled = errors.New("qmsg 未启用")

type QmsgConfig struct {
	Enabled          bool   `json:"enabled"`
	Key              string `json:"key,omitempty"`
	QQ               string `json:"qq,omitempty"`
	Bot              string `json:"bot,omitempty"`
	TimeoutSec       int    `json:"timeout_sec,omitempty"`
	MessageTemplate  string `json:"message_template,omitempty"`
	EndpointTemplate string `json:"endpoint_template,omitempty"`
}

type PublicQmsgConfig struct {
	Enabled          bool   `json:"enabled"`
	Key              string `json:"key,omitempty"`
	KeyMasked        string `json:"key_masked,omitempty"`
	QQ               string `json:"qq,omitempty"`
	Bot              string `json:"bot,omitempty"`
	TimeoutSec       int    `json:"timeout_sec"`
	MessageTemplate  string `json:"message_template"`
	EndpointTemplate string `json:"endpoint_template"`
	Configured       bool   `json:"configured"`
}

type QmsgResponse struct {
	Success bool            `json:"success"`
	Reason  string          `json:"reason"`
	Code    int             `json:"code"`
	Info    json.RawMessage `json:"info,omitempty"`
}

type SendResult struct {
	Success bool            `json:"success"`
	Reason  string          `json:"reason"`
	Code    int             `json:"code"`
	Info    json.RawMessage `json:"info,omitempty"`
	MsgID   any             `json:"msg_id,omitempty"`
}

type AccountEvent struct {
	Timestamp   time.Time
	Action      string
	Email       string
	ReasonCode  string
	Detail      string
	StorageMode string
}

type Service struct {
	mu         sync.RWMutex
	path       string
	cfg        QmsgConfig
	httpClient *http.Client
}

func DefaultQmsgConfigPath(authDir string) string {
	authDir = strings.TrimSpace(authDir)
	if authDir == "" {
		authDir = "."
	}
	return filepath.Join(authDir, qmsgConfigFileName)
}

func NewQmsgService(configPath string) (*Service, error) {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return nil, errors.New("qmsg 配置路径不能为空")
	}
	s := &Service{
		path:       configPath,
		cfg:        normalizeQmsgConfig(QmsgConfig{}),
		httpClient: &http.Client{},
	}
	if err := s.Load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Service) Load() error {
	if s == nil {
		return errors.New("qmsg 服务未初始化")
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.mu.Lock()
			s.cfg = normalizeQmsgConfig(QmsgConfig{})
			s.mu.Unlock()
			return nil
		}
		return fmt.Errorf("读取 qmsg 配置失败: %w", err)
	}
	var cfg QmsgConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("解析 qmsg 配置失败: %w", err)
	}
	s.mu.Lock()
	s.cfg = normalizeQmsgConfig(cfg)
	s.mu.Unlock()
	return nil
}

func (s *Service) Save(cfg QmsgConfig) error {
	if s == nil {
		return errors.New("qmsg 服务未初始化")
	}
	cfg = normalizeQmsgConfig(cfg)
	if cfg.Enabled && cfg.Key == "" {
		return errors.New("启用 qmsg 时必须填写 key")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("创建 qmsg 配置目录失败: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 qmsg 配置失败: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("写入 qmsg 临时配置失败: %w", err)
	}
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmp)
		return fmt.Errorf("替换 qmsg 配置失败: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("保存 qmsg 配置失败: %w", err)
	}
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	return nil
}

func (s *Service) Config() QmsgConfig {
	if s == nil {
		return normalizeQmsgConfig(QmsgConfig{})
	}
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	return normalizeQmsgConfig(cfg)
}

func (s *Service) PublicConfig(revealKey bool) PublicQmsgConfig {
	cfg := s.Config()
	publicKey := ""
	if revealKey {
		publicKey = cfg.Key
	}
	return PublicQmsgConfig{
		Enabled:          cfg.Enabled,
		Key:              publicKey,
		KeyMasked:        maskQmsgKey(cfg.Key),
		QQ:               cfg.QQ,
		Bot:              cfg.Bot,
		TimeoutSec:       cfg.TimeoutSec,
		MessageTemplate:  cfg.MessageTemplate,
		EndpointTemplate: cfg.EndpointTemplate,
		Configured:       cfg.Key != "",
	}
}

func (s *Service) Send(ctx context.Context, msg string) (*SendResult, error) {
	cfg := s.Config()
	if !cfg.Enabled {
		return nil, ErrQmsgDisabled
	}
	return s.SendWithConfig(ctx, cfg, msg)
}

func (s *Service) SendWithConfig(ctx context.Context, cfg QmsgConfig, msg string) (*SendResult, error) {
	if s == nil {
		return nil, errors.New("qmsg 服务未初始化")
	}
	cfg = normalizeQmsgConfig(cfg)
	if cfg.Key == "" {
		return nil, errors.New("qmsg key 不能为空")
	}
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return nil, errors.New("qmsg 消息内容不能为空")
	}
	endpoint, err := qmsgEndpoint(cfg)
	if err != nil {
		return nil, err
	}
	payload := map[string]string{"msg": msg}
	if cfg.QQ != "" {
		payload["qq"] = cfg.QQ
	}
	if cfg.Bot != "" {
		payload["bot"] = cfg.Bot
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化 qmsg 请求失败: %w", err)
	}
	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = DefaultQmsgTimeoutSec * time.Second
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建 qmsg 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用 qmsg 失败: %w", err)
	}
	defer res.Body.Close()
	resBody, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取 qmsg 响应失败: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		result := parseQmsgResult(resBody)
		if result != nil && strings.TrimSpace(result.Reason) != "" {
			return result, fmt.Errorf("qmsg 返回 HTTP %d: %s", res.StatusCode, strings.TrimSpace(result.Reason))
		}
		return result, fmt.Errorf("qmsg 返回 HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(resBody)))
	}
	var qres QmsgResponse
	if err := json.Unmarshal(resBody, &qres); err != nil {
		return nil, fmt.Errorf("解析 qmsg 响应失败: %w", err)
	}
	result := buildQmsgSendResult(qres)
	if !qres.Success {
		return result, fmt.Errorf("qmsg 推送失败: %s", strings.TrimSpace(qres.Reason))
	}
	return result, nil
}

func (s *Service) SendAccountEvent(ctx context.Context, event AccountEvent) (*SendResult, error) {
	cfg := s.Config()
	if !cfg.Enabled {
		return nil, ErrQmsgDisabled
	}
	msg := RenderQmsgMessage(cfg.MessageTemplate, event)
	return s.SendWithConfig(ctx, cfg, msg)
}

func RenderQmsgMessage(template string, event AccountEvent) string {
	template = strings.TrimSpace(template)
	if template == "" {
		template = DefaultQmsgMessageTemplate
	}
	actionLabel := event.Action
	switch strings.ToLower(strings.TrimSpace(event.Action)) {
	case "remove":
		actionLabel = "删除"
	case "disable":
		actionLabel = "停用"
	case "refresh_disable":
		actionLabel = "禁用刷新"
	}
	timestamp := event.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	values := map[string]string{
		"action":        actionLabel,
		"action_code":   strings.TrimSpace(event.Action),
		"email":         strings.TrimSpace(event.Email),
		"reason_code":   strings.TrimSpace(event.ReasonCode),
		"detail":        strings.TrimSpace(event.Detail),
		"storage_mode":  strings.TrimSpace(event.StorageMode),
		"timestamp":     timestamp.Local().Format("2006-01-02 15:04:05 MST"),
		"timestamp_utc": timestamp.UTC().Format(time.RFC3339),
	}
	msg := template
	for key, value := range values {
		msg = strings.ReplaceAll(msg, "{{"+key+"}}", value)
	}
	return msg
}

func normalizeQmsgConfig(cfg QmsgConfig) QmsgConfig {
	cfg.Key = strings.TrimSpace(cfg.Key)
	cfg.QQ = strings.TrimSpace(cfg.QQ)
	cfg.Bot = strings.TrimSpace(cfg.Bot)
	cfg.MessageTemplate = strings.TrimSpace(cfg.MessageTemplate)
	cfg.EndpointTemplate = strings.TrimSpace(cfg.EndpointTemplate)
	if cfg.TimeoutSec <= 0 {
		cfg.TimeoutSec = DefaultQmsgTimeoutSec
	}
	if cfg.TimeoutSec > 60 {
		cfg.TimeoutSec = 60
	}
	if cfg.MessageTemplate == "" {
		cfg.MessageTemplate = DefaultQmsgMessageTemplate
	}
	if cfg.EndpointTemplate == "" {
		cfg.EndpointTemplate = DefaultQmsgEndpointTemplate
	}
	return cfg
}

func qmsgEndpoint(cfg QmsgConfig) (string, error) {
	endpoint := strings.ReplaceAll(cfg.EndpointTemplate, "{key}", url.PathEscape(cfg.Key))
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("qmsg 接口地址无效")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("qmsg 接口地址仅支持 http/https")
	}
	return parsed.String(), nil
}

func buildQmsgSendResult(qres QmsgResponse) *SendResult {
	return &SendResult{
		Success: qres.Success,
		Reason:  qres.Reason,
		Code:    qres.Code,
		Info:    qres.Info,
		MsgID:   extractMsgID(qres.Info),
	}
}

func parseQmsgResult(body []byte) *SendResult {
	var qres QmsgResponse
	if err := json.Unmarshal(body, &qres); err != nil {
		return nil
	}
	return buildQmsgSendResult(qres)
}

func maskQmsgKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

func extractMsgID(info json.RawMessage) any {
	if len(info) == 0 {
		return nil
	}
	var obj map[string]any
	if err := json.Unmarshal(info, &obj); err != nil {
		return nil
	}
	return obj["msgId"]
}

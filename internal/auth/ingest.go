package auth

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

/**
 * IngestResult 账号导入结果
 */
type IngestResult struct {
	Added      int      `json:"added"`
	Updated    int      `json:"updated"`
	Failed     int      `json:"failed"`
	PoolTotal  int      `json:"pool_total"`
	Errors     []string `json:"errors,omitempty"`
	maxErrKeep int
}

const ingestMaxErrors = 48

func (r *IngestResult) appendErr(msg string) {
	if r.maxErrKeep == 0 {
		r.maxErrKeep = ingestMaxErrors
	}
	if len(r.Errors) >= r.maxErrKeep {
		return
	}
	r.Errors = append(r.Errors, msg)
}

/**
 * ingestNestedCredentialObjectKeys 兼容前端 tokens 与 sub2api credentials 嵌套凭据
 */
var ingestNestedCredentialObjectKeys = []string{"tokens", "credentials"}

func ingestJSONStringField(fields map[string]json.RawMessage, key string) string {
	raw, ok := fields[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

func ingestJSONObjectField(fields map[string]json.RawMessage, key string) map[string]json.RawMessage {
	raw, ok := fields[key]
	if !ok || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var nested map[string]json.RawMessage
	if err := json.Unmarshal(raw, &nested); err != nil {
		return nil
	}
	return nested
}

func fillIngestStringIfEmpty(target *string, fields map[string]json.RawMessage, key string) {
	if target == nil || strings.TrimSpace(*target) != "" {
		return
	}
	if value := ingestJSONStringField(fields, key); value != "" {
		*target = value
	}
}

func fillIngestExpireUnixIfEmpty(target *string, fields map[string]json.RawMessage, key string) {
	if target == nil || strings.TrimSpace(*target) != "" {
		return
	}
	raw, ok := fields[key]
	if !ok {
		return
	}
	var unixSeconds int64
	if err := json.Unmarshal(raw, &unixSeconds); err == nil && unixSeconds > 0 {
		*target = time.Unix(unixSeconds, 0).UTC().Format(time.RFC3339)
	}
}

func normalizeTokenFilePayload(tf *TokenFile, fields map[string]json.RawMessage) {
	if tf == nil || fields == nil {
		return
	}
	for _, key := range ingestNestedCredentialObjectKeys {
		nested := ingestJSONObjectField(fields, key)
		if nested == nil {
			continue
		}
		fillIngestStringIfEmpty(&tf.RefreshToken, nested, "refresh_token")
		fillIngestStringIfEmpty(&tf.RK, nested, "rk")
		fillIngestStringIfEmpty(&tf.AccessToken, nested, "access_token")
		fillIngestStringIfEmpty(&tf.IDToken, nested, "id_token")
		fillIngestStringIfEmpty(&tf.AccountID, nested, "account_id")
		fillIngestStringIfEmpty(&tf.AccountID, nested, "chatgpt_account_id")
		fillIngestStringIfEmpty(&tf.Email, nested, "email")
		fillIngestStringIfEmpty(&tf.Expire, nested, "expired")
		fillIngestExpireUnixIfEmpty(&tf.Expire, nested, "expires_at")
	}
	fillIngestStringIfEmpty(&tf.Email, fields, "name")
}

func parseTokenFilePayload(raw []byte) (TokenFile, error) {
	raw = bytes.TrimSpace(raw)
	var tf TokenFile
	if len(raw) == 0 {
		return tf, fmt.Errorf("空 JSON 对象")
	}
	if err := json.Unmarshal(raw, &tf); err != nil {
		return tf, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return tf, err
	}
	normalizeTokenFilePayload(&tf, fields)
	return tf, nil
}

func parseTokenFilePayloadArray(items []json.RawMessage) ([]TokenFile, error) {
	out := make([]TokenFile, 0, len(items))
	for i, item := range items {
		tf, err := parseTokenFilePayload(item)
		if err != nil {
			return nil, fmt.Errorf("解析 JSON 数组第 %d 条失败: %w", i+1, err)
		}
		out = append(out, tf)
	}
	return out, nil
}

func parseTokenFilePayloadObject(body []byte) ([]TokenFile, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, fmt.Errorf("解析 JSON 对象失败: %w", err)
	}
	if rawAccounts, ok := fields["accounts"]; ok {
		var accounts []json.RawMessage
		if err := json.Unmarshal(rawAccounts, &accounts); err != nil {
			return nil, fmt.Errorf("解析 sub2api accounts 数组失败: %w", err)
		}
		if len(accounts) == 0 {
			return nil, fmt.Errorf("sub2api accounts 数组不能为空")
		}
		return parseTokenFilePayloadArray(accounts)
	}
	one, err := parseTokenFilePayload(body)
	if err != nil {
		return nil, fmt.Errorf("解析 JSON 对象失败: %w", err)
	}
	return []TokenFile{one}, nil
}

/**
 * parseTokenFilePayloads 解析请求体：JSON 数组、单个 JSON 对象，或 NDJSON（每行一个对象）
 */
func parseTokenFilePayloads(body []byte) ([]TokenFile, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil, fmt.Errorf("空请求体")
	}
	switch body[0] {
	case '[':
		var arr []json.RawMessage
		if err := json.Unmarshal(body, &arr); err != nil {
			return nil, fmt.Errorf("解析 JSON 数组失败: %w", err)
		}
		return parseTokenFilePayloadArray(arr)
	case '{':
		return parseTokenFilePayloadObject(body)
	default:
		return parseNDJSONTokenFiles(body)
	}
}

func parseNDJSONTokenFiles(body []byte) ([]TokenFile, error) {
	lines := bytes.Split(body, []byte("\n"))
	out := make([]TokenFile, 0, len(lines))
	for i, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		var tf TokenFile
		var err error
		if tf, err = parseTokenFilePayload(line); err != nil {
			return nil, fmt.Errorf("第 %d 行 NDJSON 解析失败: %w", i+1, err)
		}
		out = append(out, tf)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("NDJSON 中无有效对象")
	}
	return out, nil
}

func sanitizeAuthFileBase(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		case r == '@', r == '+':
			b.WriteRune('_')
		default:
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "._-")
	if out == "" {
		return ""
	}
	if len(out) > 200 {
		out = out[:200]
	}
	return out
}

func ingestSyntheticAccountID(refreshToken string) string {
	h := sha256.Sum256([]byte(refreshToken))
	return "upload_" + hex.EncodeToString(h[:8])
}

/**
 * ingestIdentitySeed 生成合成 account_id 的熵源；无 rk 时用 access_token / id_token，避免多条上传撞同一 id
 */
func ingestIdentitySeed(acc *Account) string {
	if acc == nil {
		return ""
	}
	if s := strings.TrimSpace(acc.Token.RefreshToken); s != "" {
		return s
	}
	if s := strings.TrimSpace(acc.Token.AccessToken); s != "" {
		return s
	}
	return strings.TrimSpace(acc.Token.IDToken)
}

func ingestLogIdent(acc *Account) string {
	if acc == nil {
		return ""
	}
	if s := strings.TrimSpace(acc.Token.Email); s != "" {
		return s
	}
	if s := strings.TrimSpace(acc.Token.AccountID); s != "" {
		return s
	}
	return acc.FilePath
}

/**
 * ensureIngestDBIdentity 数据库模式下保证 account_id 与 email 至少有一个非空，以便 upsert 与 FilePath 稳定
 */
func ensureIngestDBIdentity(acc *Account) {
	if strings.TrimSpace(acc.Token.AccountID) == "" && strings.TrimSpace(acc.Token.Email) == "" {
		seed := ingestIdentitySeed(acc)
		if seed == "" {
			seed = "empty"
		}
		acc.Token.AccountID = ingestSyntheticAccountID(seed)
	}
}

func (m *Manager) ingestFilePathForAccount(acc *Account) string {
	if m.db != nil {
		aid := strings.TrimSpace(acc.Token.AccountID)
		em := strings.TrimSpace(acc.Token.Email)
		if !acc.HasRefreshToken() {
			if em != "" {
				return "db:" + em
			}
			if aid != "" {
				return "db:" + aid
			}
			return "db:" + ingestSyntheticAccountID(ingestIdentitySeed(acc))
		}
		if aid != "" {
			return "db:" + aid
		}
		if em != "" {
			return "db:" + em
		}
		return "db:" + ingestSyntheticAccountID(ingestIdentitySeed(acc))
	}
	base := sanitizeAuthFileBase(acc.Token.Email)
	if base == "" {
		base = sanitizeAuthFileBase(acc.Token.AccountID)
	}
	if base == "" {
		base = ingestSyntheticAccountID(ingestIdentitySeed(acc))
	}
	return filepath.Join(m.authDir, base+".json")
}

func (m *Manager) IngestAccountsFromJSON(body []byte) (IngestResult, error) {
	if m.db == nil && strings.TrimSpace(m.authDir) == "" {
		return IngestResult{}, fmt.Errorf("未配置 auth-dir 且未启用数据库，无法导入")
	}
	tokens, err := parseTokenFilePayloads(body)
	if err != nil {
		return IngestResult{}, err
	}
	if m.db != nil {
		m.importMu.Lock()
		defer m.importMu.Unlock()
	}

	var res IngestResult
	for i, tf := range tokens {
		acc, aerr := accountFromTokenFile(&tf, "")
		if aerr != nil {
			res.Failed++
			res.appendErr(fmt.Sprintf("#%d: %v", i+1, aerr))
			log.Warnf("账号上传: 跳过第 %d/%d 条: %v", i+1, len(tokens), aerr)
			continue
		}
		if m.db != nil {
			ensureIngestDBIdentity(acc)
		}
		acc.FilePath = m.ingestFilePathForAccount(acc)

		m.mu.Lock()
		if ex, ok := m.accountIndex[acc.FilePath]; ok {
			ex.UpdateToken(acc.TokenSnapshot())
			m.mu.Unlock()
			m.enqueueSave(ex)
			res.Updated++
			log.Debugf("账号上传: 更新 ident=%s path=%s has_refresh_token=%t", ingestLogIdent(ex), ex.FilePath, ex.HasRefreshToken())
		} else {
			m.accounts = append(m.accounts, acc)
			m.accountIndex[acc.FilePath] = acc
			m.publishSnapshot()
			m.mu.Unlock()
			m.enqueueSave(acc)
			res.Added++
			log.Debugf("账号上传: 新增 ident=%s path=%s has_refresh_token=%t", ingestLogIdent(acc), acc.FilePath, acc.HasRefreshToken())
		}
	}
	if res.Added+res.Updated > 0 {
		m.InvalidateSelectorCache()
	}
	m.mu.RLock()
	res.PoolTotal = len(m.accounts)
	m.mu.RUnlock()
	if res.Added+res.Updated+res.Failed > 0 {
		log.Debugf("账号上传汇总: 新增=%d 更新=%d 失败=%d 号池合计=%d", res.Added, res.Updated, res.Failed, res.PoolTotal)
	}
	return res, nil
}

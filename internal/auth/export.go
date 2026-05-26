package auth

import (
	"fmt"
	"strings"
	"time"
)

const (
	Sub2APIExportFormatExport = "sub2api-export"
	Sub2APIExportFormatArray  = "sub2api-array"
)

/**
 * Sub2APICredentials sub2api 导出凭据结构
 */
type Sub2APICredentials struct {
	RefreshToken     string `json:"refresh_token,omitempty"`
	AccessToken      string `json:"access_token,omitempty"`
	IDToken          string `json:"id_token,omitempty"`
	AccountID        string `json:"account_id,omitempty"`
	ChatgptAccountID string `json:"chatgpt_account_id,omitempty"`
	Email            string `json:"email,omitempty"`
	Expired          string `json:"expired,omitempty"`
	ExpiresAt        int64  `json:"expires_at,omitempty"`
}

/**
 * Sub2APIAccount sub2api 单条账号导出结构
 */
type Sub2APIAccount struct {
	Name        string             `json:"name"`
	Platform    string             `json:"platform"`
	Type        string             `json:"type"`
	Credentials Sub2APICredentials `json:"credentials"`
}

/**
 * Sub2APIExportFile sub2api 完整导出文件结构
 */
type Sub2APIExportFile struct {
	ExportedAt string           `json:"exported_at"`
	Proxies    []any            `json:"proxies"`
	Accounts   []Sub2APIAccount `json:"accounts"`
}

/**
 * ExportAccountFailure 单条账号导出失败信息
 */
type ExportAccountFailure struct {
	Email string `json:"email"`
	Error string `json:"error"`
}

/**
 * ExportAccountsSub2APIResult 批量 sub2api 导出结果
 */
type ExportAccountsSub2APIResult struct {
	Accounts  []Sub2APIAccount
	NotFound  []string
	Failed    []ExportAccountFailure
	Exported  int
	ExportedAt time.Time
}

func tokenExpireToUnixSeconds(expire string) int64 {
	expire = strings.TrimSpace(expire)
	if expire == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339, expire)
	if err != nil {
		return 0
	}
	return t.Unix()
}

func accountToSub2API(acc *Account) (Sub2APIAccount, error) {
	if acc == nil {
		return Sub2APIAccount{}, fmt.Errorf("账号不存在")
	}
	token := acc.TokenSnapshot()
	refreshToken := strings.TrimSpace(token.RefreshToken)
	accessToken := strings.TrimSpace(token.AccessToken)
	idToken := strings.TrimSpace(token.IDToken)
	if refreshToken == "" && accessToken == "" && idToken == "" {
		return Sub2APIAccount{}, fmt.Errorf("缺少可导出的凭据")
	}

	email := strings.TrimSpace(token.Email)
	accountID := strings.TrimSpace(token.AccountID)
	name := email
	if name == "" {
		name = accountID
	}
	if name == "" {
		return Sub2APIAccount{}, fmt.Errorf("缺少邮箱或 account_id")
	}

	expire := strings.TrimSpace(token.Expire)
	credentials := Sub2APICredentials{
		RefreshToken: refreshToken,
		AccessToken:  accessToken,
		IDToken:      idToken,
		AccountID:    accountID,
		Email:        email,
		Expired:      expire,
		ExpiresAt:    tokenExpireToUnixSeconds(expire),
	}
	if accountID != "" {
		credentials.ChatgptAccountID = accountID
	}

	return Sub2APIAccount{
		Name:        name,
		Platform:    "openai",
		Type:        "oauth",
		Credentials: credentials,
	}, nil
}

/**
 * ExportAccountsSub2API 按邮箱列表导出 sub2api 账号记录
 */
func (m *Manager) ExportAccountsSub2API(emails []string) ExportAccountsSub2APIResult {
	result := ExportAccountsSub2APIResult{
		Accounts:   make([]Sub2APIAccount, 0, len(emails)),
		NotFound:   make([]string, 0),
		Failed:     make([]ExportAccountFailure, 0),
		ExportedAt: time.Now().UTC(),
	}
	seen := make(map[string]struct{}, len(emails))
	for _, rawEmail := range emails {
		email := strings.TrimSpace(rawEmail)
		if email == "" {
			continue
		}
		key := strings.ToLower(email)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		acc := m.FindAccountByIdentifier(email, "")
		if acc == nil {
			result.NotFound = append(result.NotFound, email)
			continue
		}
		record, err := accountToSub2API(acc)
		if err != nil {
			result.Failed = append(result.Failed, ExportAccountFailure{
				Email: email,
				Error: err.Error(),
			})
			continue
		}
		result.Accounts = append(result.Accounts, record)
	}
	result.Exported = len(result.Accounts)
	return result
}

/**
 * BuildSub2APIExportFile 构造 sub2api 完整导出文件
 */
func BuildSub2APIExportFile(accounts []Sub2APIAccount, exportedAt time.Time) Sub2APIExportFile {
	return Sub2APIExportFile{
		ExportedAt: exportedAt.UTC().Format(time.RFC3339Nano),
		Proxies:    []any{},
		Accounts:   accounts,
	}
}

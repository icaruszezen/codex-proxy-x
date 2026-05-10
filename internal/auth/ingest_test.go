package auth

import "testing"

func TestParseTokenFilePayloadsStandardFormats(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "object",
			body: `{"email":"a@example.com","refresh_token":"rt_a","type":"codex"}`,
		},
		{
			name: "array",
			body: `[{"email":"a@example.com","refresh_token":"rt_a"},{"email":"b@example.com","rk":"rk_b"}]`,
		},
		{
			name: "ndjson",
			body: "# comment\n{\"email\":\"a@example.com\",\"refresh_token\":\"rt_a\"}\n{\"email\":\"b@example.com\",\"id_token\":\"id_b\"}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := parseTokenFilePayloads([]byte(tt.body))
			if err != nil {
				t.Fatalf("parseTokenFilePayloads() error = %v", err)
			}
			if len(tokens) == 0 {
				t.Fatal("parseTokenFilePayloads() returned no tokens")
			}
		})
	}
}

func TestParseTokenFilePayloadsSub2APISingleObject(t *testing.T) {
	body := `{
	  "name": "single@example.com",
	  "platform": "openai",
	  "type": "oauth",
	  "credentials": {
	    "refresh_token": "rt_single"
	  }
	}`

	tokens, err := parseTokenFilePayloads([]byte(body))
	if err != nil {
		t.Fatalf("parseTokenFilePayloads() error = %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("len(tokens) = %d, want 1", len(tokens))
	}
	if tokens[0].Email != "single@example.com" {
		t.Fatalf("tokens[0].Email = %q", tokens[0].Email)
	}
	if tokens[0].RefreshToken != "rt_single" {
		t.Fatalf("tokens[0].RefreshToken = %q", tokens[0].RefreshToken)
	}
}

func TestParseTokenFilePayloadsSub2APICredentials(t *testing.T) {
	body := `[
	  {
	    "name": "user-a@example.com",
	    "platform": "openai",
	    "type": "oauth",
	    "group_ids": [1, 2],
	    "credentials": {
	      "refresh_token": "rt_a",
	      "access_token": "at_a",
	      "id_token": "id_a",
	      "account_id": "acct_a",
	      "expired": "2026-01-01T00:00:00Z"
	    }
	  },
	  {
	    "name": "user-b@example.com",
	    "credentials": {
	      "rk": "rk_b"
	    }
	  }
	]`

	tokens, err := parseTokenFilePayloads([]byte(body))
	if err != nil {
		t.Fatalf("parseTokenFilePayloads() error = %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("len(tokens) = %d, want 2", len(tokens))
	}
	if tokens[0].Email != "user-a@example.com" {
		t.Fatalf("tokens[0].Email = %q", tokens[0].Email)
	}
	if tokens[0].RefreshToken != "rt_a" {
		t.Fatalf("tokens[0].RefreshToken = %q", tokens[0].RefreshToken)
	}
	if tokens[0].AccessToken != "at_a" || tokens[0].IDToken != "id_a" {
		t.Fatalf("nested access/id tokens not promoted: %#v", tokens[0])
	}
	if tokens[0].AccountID != "acct_a" || tokens[0].Expire != "2026-01-01T00:00:00Z" {
		t.Fatalf("nested identity fields not promoted: %#v", tokens[0])
	}
	if tokens[1].Email != "user-b@example.com" {
		t.Fatalf("tokens[1].Email = %q", tokens[1].Email)
	}
	if tokens[1].RK != "rk_b" {
		t.Fatalf("tokens[1].RK = %q", tokens[1].RK)
	}
}

func TestParseTokenFilePayloadTopLevelWins(t *testing.T) {
	body := `{
	  "email": "top@example.com",
	  "refresh_token": "rt_top",
	  "credentials": {
	    "email": "nested@example.com",
	    "refresh_token": "rt_nested",
	    "access_token": "at_nested"
	  }
	}`

	tokens, err := parseTokenFilePayloads([]byte(body))
	if err != nil {
		t.Fatalf("parseTokenFilePayloads() error = %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("len(tokens) = %d, want 1", len(tokens))
	}
	if tokens[0].Email != "top@example.com" {
		t.Fatalf("tokens[0].Email = %q", tokens[0].Email)
	}
	if tokens[0].RefreshToken != "rt_top" {
		t.Fatalf("tokens[0].RefreshToken = %q", tokens[0].RefreshToken)
	}
	if tokens[0].AccessToken != "at_nested" {
		t.Fatalf("tokens[0].AccessToken = %q", tokens[0].AccessToken)
	}
}

func TestParseTokenFilePayloadsSub2APIRecordWithoutCredentialFailsAccountValidation(t *testing.T) {
	tokens, err := parseTokenFilePayloads([]byte(`[{"name":"no-token@example.com","credentials":{}}]`))
	if err != nil {
		t.Fatalf("parseTokenFilePayloads() error = %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("len(tokens) = %d, want 1", len(tokens))
	}
	if _, err := accountFromTokenFile(&tokens[0], ""); err == nil {
		t.Fatal("accountFromTokenFile() error = nil, want credential validation error")
	}
}

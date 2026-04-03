package wechat

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExchangeMiniProgramCode(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sns/jscode2session" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("appid"); got != "wx-app-id" {
			t.Fatalf("unexpected appid: %s", got)
		}
		if got := query.Get("secret"); got != "wx-secret" {
			t.Fatalf("unexpected secret: %s", got)
		}
		if got := query.Get("js_code"); got != "login-code" {
			t.Fatalf("unexpected js_code: %s", got)
		}
		if got := query.Get("grant_type"); got != "authorization_code" {
			t.Fatalf("unexpected grant_type: %s", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"openid":"openid-123","session_key":"session-key-123","unionid":"unionid-123"}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		AppID:      "wx-app-id",
		AppSecret:  "wx-secret",
		APIBaseURL: server.URL,
	})

	resp, err := client.ExchangeMiniProgramCode(context.Background(), "login-code")
	if err != nil {
		t.Fatalf("ExchangeMiniProgramCode returned error: %v", err)
	}
	if resp.OpenID != "openid-123" {
		t.Fatalf("unexpected openid: %s", resp.OpenID)
	}
	if resp.SessionKey != "session-key-123" {
		t.Fatalf("unexpected session_key: %s", resp.SessionKey)
	}
	if resp.UnionID != "unionid-123" {
		t.Fatalf("unexpected unionid: %s", resp.UnionID)
	}
}

func TestExchangeMiniProgramCodeAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":40029,"errmsg":"invalid code"}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		AppID:      "wx-app-id",
		AppSecret:  "wx-secret",
		APIBaseURL: server.URL,
	})

	_, err := client.ExchangeMiniProgramCode(context.Background(), "bad-code")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != 40029 {
		t.Fatalf("unexpected error code: %d", apiErr.Code)
	}
	if apiErr.Message != "invalid code" {
		t.Fatalf("unexpected error message: %s", apiErr.Message)
	}
}

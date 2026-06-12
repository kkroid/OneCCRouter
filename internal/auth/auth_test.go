package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckTokenAvailable(t *testing.T) {
	dir := t.TempDir()
	tf := filepath.Join(dir, "token")

	tm, err := NewTokenManager(tf, "")
	if err != nil {
		t.Fatal(err)
	}
	if tm.CheckTokenAvailable() {
		t.Error("expected false for missing token file")
	}

	os.WriteFile(tf, []byte("ghu_test"), 0600)
	if !tm.CheckTokenAvailable() {
		t.Error("expected true after writing token")
	}

	os.WriteFile(tf, []byte(""), 0600)
	if tm.CheckTokenAvailable() {
		t.Error("expected false for empty token file")
	}
}

func TestTokenCache(t *testing.T) {
	tm, err := NewTokenManager("/nonexistent/token", "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = tm.GetToken()
	if err == nil {
		t.Error("expected error for missing token file")
	}

	tm.cachedToken = "cached-token"
	tm.cachedExp = time.Now().Unix() + 3600

	token, err := tm.GetToken()
	if err != nil {
		t.Fatal(err)
	}
	if token != "cached-token" {
		t.Errorf("expected cached token, got %s", token)
	}
}

func TestTokenCacheExpiry(t *testing.T) {
	tm, err := NewTokenManager("/nonexistent/token", "")
	if err != nil {
		t.Fatal(err)
	}

	tm.cachedToken = "expired-token"
	tm.cachedExp = time.Now().Unix() - 200

	_, err = tm.GetToken()
	if err == nil {
		t.Error("expected error when refreshing with no token file")
	}
}

func TestGetAPIBase(t *testing.T) {
	tm, _ := NewTokenManager("/x", "")
	if tm.GetAPIBase() != defaultAPIBase {
		t.Errorf("expected default API base: %s", defaultAPIBase)
	}

	tm.cachedAPIBase = "https://api.individual.githubcopilot.com"
	if tm.GetAPIBase() != "https://api.individual.githubcopilot.com" {
		t.Error("expected cached API base")
	}
}

func TestTokenManagerProxyAddr(t *testing.T) {
	tm, err := NewTokenManager("/x", "127.0.0.1:1082")
	if err != nil {
		t.Fatal(err)
	}
	if tm.httpClient == nil {
		t.Error("httpClient should be set")
	}
	if tm.httpClient.Timeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", tm.httpClient.Timeout)
	}
}

func TestMakeTransportNoProxy(t *testing.T) {
	transport, err := makeTransport("")
	if err != nil {
		t.Fatal(err)
	}
	if transport.DialContext != nil {
		t.Error("expected nil dial context when no proxy")
	}
}

func TestMakeTransportWithProxy(t *testing.T) {
	transport, err := makeTransport("127.0.0.1:1082")
	if err != nil {
		t.Fatal(err)
	}
	if transport.DialContext == nil {
		t.Error("expected non-nil dial context with proxy")
	}
}

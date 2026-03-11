package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// setupTestConfig initialises appConfig with a known password for tests.
func setupTestConfig(t *testing.T, password string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	appConfig = AppConfig{PasswordHash: string(hash)}
	configPath = t.TempDir() + "/config.json"
}

// ----- sanitizeHeaderValue -----

func TestSanitizeHeaderValue_RemovesNewlines(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"safe-id", "safe-id"},
		{"id\ninjected", "idinjected"},
		{"id\rinjected", "idinjected"},
		{"id\r\ninjected", "idinjected"},
		{"id\x00null", "idnull"},
		{"normal-123", "normal-123"},
	}
	for _, tt := range tests {
		got := sanitizeHeaderValue(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeHeaderValue(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

// ----- requestIDMiddleware -----

func TestRequestIDMiddleware_PropagatesCleanID(t *testing.T) {
	handler := requestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "abc-123\ninjected")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	got := rr.Header().Get("X-Request-ID")
	if strings.Contains(got, "\n") || strings.Contains(got, "\r") {
		t.Errorf("response X-Request-ID contains control chars: %q", got)
	}
	if got != "abc-123injected" {
		t.Errorf("X-Request-ID = %q; want %q", got, "abc-123injected")
	}
}

// ----- checkLoginRateLimit -----

func TestCheckLoginRateLimit_AllowsUpToMax(t *testing.T) {
	// Use a unique IP so as not to interfere with other tests.
	ip := "192.0.2.1"
	loginMu.Lock()
	delete(loginAttempts, ip)
	loginMu.Unlock()

	for i := 0; i < loginMaxAttempts; i++ {
		if !checkLoginRateLimit(ip) {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	if checkLoginRateLimit(ip) {
		t.Fatal("attempt beyond max should be blocked")
	}
}

func TestCheckLoginRateLimit_ResetsAfterWindow(t *testing.T) {
	ip := "192.0.2.2"
	loginMu.Lock()
	loginAttempts[ip] = &loginAttempt{count: loginMaxAttempts, windowEnd: time.Now().Add(-time.Second)}
	loginMu.Unlock()

	if !checkLoginRateLimit(ip) {
		t.Fatal("should be allowed after window expires")
	}
}

// ----- handleLogin -----

func TestHandleLogin_Success(t *testing.T) {
	setupTestConfig(t, "correctpassword")

	body, _ := json.Marshal(map[string]string{"password": "correctpassword"})
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
	req.RemoteAddr = "10.0.0.1:1234"
	// Reset rate-limit state for this IP.
	loginMu.Lock()
	delete(loginAttempts, "10.0.0.1:1234")
	loginMu.Unlock()

	rr := httptest.NewRecorder()
	handleLogin(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", rr.Code)
	}
}

func TestHandleLogin_WrongPassword(t *testing.T) {
	setupTestConfig(t, "correctpassword")

	body, _ := json.Marshal(map[string]string{"password": "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
	req.RemoteAddr = "10.0.0.2:1234"
	loginMu.Lock()
	delete(loginAttempts, "10.0.0.2:1234")
	loginMu.Unlock()

	rr := httptest.NewRecorder()
	handleLogin(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", rr.Code)
	}
}

func TestHandleLogin_RateLimited(t *testing.T) {
	setupTestConfig(t, "correctpassword")
	ip := "10.0.0.3:5678"

	loginMu.Lock()
	loginAttempts[ip] = &loginAttempt{count: loginMaxAttempts, windowEnd: time.Now().Add(loginRateWindow)}
	loginMu.Unlock()

	body, _ := json.Marshal(map[string]string{"password": "correctpassword"})
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
	req.RemoteAddr = ip

	rr := httptest.NewRecorder()
	handleLogin(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d; want 429", rr.Code)
	}
}

func TestHandleLogin_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/login", nil)
	rr := httptest.NewRecorder()
	handleLogin(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d; want 405", rr.Code)
	}
}

// ----- handleSetPassword -----

func TestHandleSetPassword_Success(t *testing.T) {
	setupTestConfig(t, "oldpassword")

	body, _ := json.Marshal(map[string]string{
		"currentPassword": "oldpassword",
		"newPassword":     "newpassword123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/set-password", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	handleSetPassword(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", rr.Code)
	}

	// New password must work.
	if err := bcrypt.CompareHashAndPassword([]byte(appConfig.PasswordHash), []byte("newpassword123")); err != nil {
		t.Error("new password hash does not match")
	}
}

func TestHandleSetPassword_WrongCurrent(t *testing.T) {
	setupTestConfig(t, "oldpassword")

	body, _ := json.Marshal(map[string]string{
		"currentPassword": "wrong",
		"newPassword":     "newpassword123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/set-password", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	handleSetPassword(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", rr.Code)
	}
}

func TestHandleSetPassword_TooShort(t *testing.T) {
	setupTestConfig(t, "oldpassword")

	body, _ := json.Marshal(map[string]string{
		"currentPassword": "oldpassword",
		"newPassword":     "short",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/set-password", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	handleSetPassword(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", rr.Code)
	}
}

// ----- saveConfig uses 0600 -----

func TestSaveConfig_FilePermissions(t *testing.T) {
	setupTestConfig(t, "testpass")

	if err := saveConfig(configPath); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}

	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("config file permissions = %04o; want 0600", perm)
	}
}

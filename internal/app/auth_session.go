package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

const (
	maxFailedAttempts    = 5
	bruteForceWindowSize = 15 * time.Minute
	bruteForceBlockTime  = 30 * time.Minute
)

type loginAttempt struct {
	failedCount  int
	lastFailure  time.Time
	blockedUntil time.Time
}

var (
	loginAttempts = make(map[string]*loginAttempt)
	attemptMutex  sync.Mutex
)

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For first (for proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if ip := strings.TrimSpace(ips[0]); ip != "" {
			return ip
		}
	}

	// Check X-Real-IP (for some proxies)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if ip := strings.TrimSpace(xri); ip != "" {
			return ip
		}
	}

	// Fall back to RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func recordLoginAttempt(ip string, success bool) {
	attemptMutex.Lock()
	defer attemptMutex.Unlock()

	attempt, exists := loginAttempts[ip]
	if !exists {
		attempt = &loginAttempt{}
		loginAttempts[ip] = attempt
	}

	if success {
		// Clear on successful login
		attempt.failedCount = 0
		attempt.blockedUntil = time.Time{}
		return
	}

	// Track failed attempt
	now := time.Now()
	if now.Before(attempt.blockedUntil) {
		// Already blocked, just update the timestamp
		attempt.lastFailure = now
		return
	}

	// Reset counter if window has expired
	if now.Sub(attempt.lastFailure) > bruteForceWindowSize {
		attempt.failedCount = 0
	}

	attempt.failedCount++
	attempt.lastFailure = now

	// Block after max attempts
	if attempt.failedCount >= maxFailedAttempts {
		attempt.blockedUntil = now.Add(bruteForceBlockTime)
	}
}

func isClientBlocked(ip string) bool {
	attemptMutex.Lock()
	defer attemptMutex.Unlock()

	attempt, exists := loginAttempts[ip]
	if !exists {
		return false
	}

	if time.Now().After(attempt.blockedUntil) {
		return false
	}

	return true
}

func handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	cfg := getConfig()
	if cfg.PasswordHash == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"success":        true,
			"token":          "",
			"passwordNeeded": false,
		})
		return
	}

	clientIP := getClientIP(r)

	// Check if client is rate limited
	if isClientBlocked(clientIP) {
		LoggerWith(logrus.Fields{
			"event":      "login_blocked",
			"ip":         clientIP,
			"reason":     "too_many_attempts",
			"request_id": requestIDFromContext(r.Context()),
		}).Warn("login attempt from blocked IP")
		writeError(w, http.StatusTooManyRequests, "too many login attempts")
		return
	}

	var req LoginRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		recordLoginAttempt(clientIP, false)
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(cfg.PasswordHash), []byte(req.Password)); err != nil {
		recordLoginAttempt(clientIP, false)
		LoggerWith(logrus.Fields{
			"event":      "login_failed",
			"ip":         clientIP,
			"request_id": requestIDFromContext(r.Context()),
		}).Warn("failed login attempt")
		writeError(w, http.StatusUnauthorized, "invalid password")
		return
	}

	recordLoginAttempt(clientIP, true)
	LoggerWith(logrus.Fields{
		"event":      "login_success",
		"ip":         clientIP,
		"request_id": requestIDFromContext(r.Context()),
	}).Info("successful login")

	token := createSessionToken()
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "token": token, "passwordNeeded": true})
}

func handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	token := extractBearerToken(r)
	if token != "" {
		removeSession(token)
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	cfg := getConfig()
	authEnabled := cfg.PasswordHash != ""
	authenticated := !authEnabled
	if authEnabled {
		authenticated = validateSessionToken(extractBearerToken(r))
	}

	writeJSON(w, http.StatusOK, AuthStatusResponse{
		AuthEnabled:        authEnabled,
		Authenticated:      authenticated,
		TaskPollIntervalMs: cfg.TaskPollIntervalMs,
	})
}

func withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isAuthEnabled() {
			next(w, r)
			return
		}

		token := extractAuthToken(r)
		if token == "" || !validateSessionToken(token) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		next(w, r)
	}
}

func createSessionToken() string {
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	token := hex.EncodeToString(raw)

	sessionMu.Lock()
	sessions[token] = time.Now().Add(tokenTTL)
	sessionMu.Unlock()
	return token
}

func validateSessionToken(token string) bool {
	if token == "" {
		return false
	}

	sessionMu.Lock()
	defer sessionMu.Unlock()

	expiresAt, ok := sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(expiresAt) {
		delete(sessions, token)
		return false
	}
	return true
}

func removeSession(token string) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	delete(sessions, token)
}

func extractBearerToken(r *http.Request) string {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func extractAuthToken(r *http.Request) string {
	if token := extractBearerToken(r); token != "" {
		return token
	}

	// Media requests from <img>/<video> cannot attach Authorization headers reliably.
	if r.Method == http.MethodGet && r.URL.Path == "/api/media" {
		return strings.TrimSpace(r.URL.Query().Get("token"))
	}

	return ""
}

func generateID(prefix string) string {
	raw := make([]byte, 6)
	_, _ = rand.Read(raw)
	return fmt.Sprintf("%s_%d_%s", prefix, time.Now().UnixNano(), hex.EncodeToString(raw))
}

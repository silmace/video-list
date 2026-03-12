package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

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

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(cfg.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid password")
		return
	}

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

package k3client

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Session is the cached K3 login result, persisted per profile so short-lived
// CLI invocations don't re-login every time. The session id is a bearer token,
// so the file is written 0600. LoginAt enables future age-based refresh.
type Session struct {
	SessionID string `json:"sessionId"`
	AcctID    string `json:"acctId"`
	LoginAt   int64  `json:"loginAt"`
}

func loadSession(path string) (Session, bool) {
	if path == "" {
		return Session{}, false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Session{}, false
	}
	var s Session
	if json.Unmarshal(b, &s) != nil {
		return Session{}, false
	}
	return s, true
}

func saveSession(path string, s Session) {
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o600)
}

func clearSessionFile(path string) {
	if path != "" {
		_ = os.Remove(path)
	}
}

// maskSession redacts a session id for display.
func maskSession(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) <= 8 {
		return "****"
	}
	return string(r[:4]) + "..." + string(r[len(r)-4:])
}

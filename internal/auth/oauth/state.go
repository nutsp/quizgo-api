package oauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	stateCookieName = "oauth_state"
	stateTTL        = 10 * time.Minute
)

type statePayload struct {
	Nonce    string `json:"nonce"`
	Redirect string `json:"redirect"`
	Expires  int64  `json:"exp"`
}

// StateManager signs OAuth state and stores it in an httpOnly cookie.
type StateManager struct {
	secret []byte
}

func NewStateManager(secret string) *StateManager {
	return &StateManager{secret: []byte(secret)}
}

func (m *StateManager) Create(w http.ResponseWriter, redirect string) (string, error) {
	nonce, err := randomString(32)
	if err != nil {
		return "", err
	}

	payload := statePayload{
		Nonce:    nonce,
		Redirect: SafeRedirect(redirect),
		Expires:  time.Now().UTC().Add(stateTTL).Unix(),
	}

	state, err := m.sign(payload)
	if err != nil {
		return "", err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   int(stateTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	return state, nil
}

func (m *StateManager) Validate(w http.ResponseWriter, r *http.Request, state string) (string, error) {
	cookie, err := r.Cookie(stateCookieName)
	if err != nil || cookie.Value == "" {
		return "", errors.New("missing oauth state cookie")
	}
	if cookie.Value != state {
		return "", errors.New("oauth state mismatch")
	}

	payload, err := m.verify(state)
	if err != nil {
		return "", err
	}
	if time.Now().UTC().Unix() > payload.Expires {
		return "", errors.New("oauth state expired")
	}

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	return SafeRedirect(payload.Redirect), nil
}

func (m *StateManager) sign(payload statePayload) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	sig := m.mac(encoded)
	return encoded + "." + sig, nil
}

func (m *StateManager) verify(state string) (*statePayload, error) {
	parts := strings.Split(state, ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid oauth state format")
	}

	expectedSig := m.mac(parts[0])
	if !hmac.Equal([]byte(expectedSig), []byte(parts[1])) {
		return nil, errors.New("invalid oauth state signature")
	}

	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}

	var payload statePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (m *StateManager) mac(message string) string {
	h := hmac.New(sha256.New, m.secret)
	h.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b)[:n], nil
}

// SafeRedirect allows only internal paths starting with /.
func SafeRedirect(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return "/exams"
	}
	return path
}

func BuildFrontendCallbackURL(frontendURL, token, redirect string) string {
	base := strings.TrimRight(frontendURL, "/")
	q := url.Values{}
	q.Set("token", token)
	if redirect != "" && redirect != "/exams" {
		q.Set("redirect", redirect)
	}
	return base + "/auth/callback?" + q.Encode()
}

func BuildFrontendErrorURL(frontendURL string) string {
	return fmt.Sprintf("%s/login?error=oauth_failed", strings.TrimRight(frontendURL, "/"))
}

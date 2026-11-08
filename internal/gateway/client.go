package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is the admin client of the LiteLLM Proxy. It holds the master key, so it lives on the
// worker side only. It issues one virtual key per Agent Session: restricted to one model, with a
// budget and an expiry, and revoked when the session ends.
type Client struct {
	BaseURL   string // e.g. http://localhost:4000
	MasterKey string
	HTTP      *http.Client
}

// SessionKey is a virtual key for one Agent Session. Key is a secret: never log it.
type SessionKey struct {
	Key       string
	Alias     string
	ExpiresAt time.Time
}

// String redacts the key.
func (k SessionKey) String() string { return "SessionKey{alias=" + k.Alias + "}" }

// Alias returns the key alias used for a session. Only the alias is stored in the database.
func Alias(sessionID string) string { return "yari-session-" + sessionID }

// ErrBudgetExhausted marks a call that LiteLLM rejected because the session's budget ran out.
var ErrBudgetExhausted = errors.New("gateway: session budget exhausted")

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	u := strings.TrimRight(c.BaseURL, "/") + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.MasterKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	h := c.HTTP
	if h == nil {
		h = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := h.Do(req)
	if err != nil {
		return fmt.Errorf("gateway: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		// The body can echo request fields, so keep only a short, key-free excerpt.
		return fmt.Errorf("gateway: %s %s: status %d: %s", method, path, resp.StatusCode, scrub(string(data), c.MasterKey))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("gateway: %s %s: decode: %w", method, path, err)
	}
	return nil
}

func scrub(s string, secrets ...string) string {
	for _, sec := range secrets {
		if sec != "" {
			s = strings.ReplaceAll(s, sec, "[redacted]")
		}
	}
	if len(s) > 300 {
		s = s[:300] + "..."
	}
	return s
}

// CreateSessionKey creates a virtual key restricted to one model, with a budget in USD and a
// lifetime.
func (c *Client) CreateSessionKey(ctx context.Context, sessionID, model string, budgetUSD float64, ttl time.Duration) (SessionKey, error) {
	alias := Alias(sessionID)
	var resp struct {
		Key     string `json:"key"`
		Expires string `json:"expires"`
	}
	err := c.do(ctx, http.MethodPost, "/key/generate", nil, map[string]any{
		"key_alias":         alias,
		"models":            []string{model},
		"max_budget":        budgetUSD,
		"duration":          fmt.Sprintf("%ds", int(ttl.Seconds())),
		"metadata":          map[string]string{"yari_session": sessionID},
		"permissions":       map[string]any{},
		"send_invite_email": false,
	}, &resp)
	if err != nil {
		return SessionKey{}, err
	}
	if resp.Key == "" {
		return SessionKey{}, errors.New("gateway: key/generate returned no key")
	}
	k := SessionKey{Key: resp.Key, Alias: alias}
	if t, err := time.Parse(time.RFC3339, resp.Expires); err == nil {
		k.ExpiresAt = t
	}
	return k, nil
}

type keyInfo struct {
	KeyAlias string  `json:"key_alias"`
	Spend    float64 `json:"spend"`
}

// Spend returns the USD spent by the key with the given alias.
func (c *Client) Spend(ctx context.Context, alias string) (float64, error) {
	var resp struct {
		Keys []json.RawMessage `json:"keys"`
	}
	q := url.Values{"key_alias": {alias}, "return_full_object": {"true"}}
	if err := c.do(ctx, http.MethodGet, "/key/list", q, nil, &resp); err != nil {
		return 0, err
	}
	for _, raw := range resp.Keys {
		var ki keyInfo
		if json.Unmarshal(raw, &ki) == nil && ki.KeyAlias == alias {
			return ki.Spend, nil
		}
	}
	return 0, fmt.Errorf("gateway: no key with alias %s", alias)
}

// Revoke deletes the key with the given alias. Revoking a key that is already gone succeeds.
func (c *Client) Revoke(ctx context.Context, alias string) error {
	err := c.do(ctx, http.MethodPost, "/key/delete", nil, map[string]any{"key_aliases": []string{alias}}, nil)
	if err != nil && (strings.Contains(err.Error(), "status 404") || strings.Contains(err.Error(), "not found")) {
		return nil
	}
	return err
}

// IsBudgetExhausted reports whether a message from the agent or the proxy says the budget ran out.
func IsBudgetExhausted(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "exceededbudget") || strings.Contains(m, "budget has been exceeded") ||
		strings.Contains(m, "crossed spend") || strings.Contains(m, "max budget")
}

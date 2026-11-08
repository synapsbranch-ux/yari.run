package gateway_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/synapsbranch-ux/yari.run/internal/gateway"
)

func TestSessionKeyLifecycleAgainstAFakeProxy(t *testing.T) {
	var generate map[string]any
	var deleted map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer master-fake", r.Header.Get("Authorization"), "admin calls use the master key")
		body, _ := io.ReadAll(r.Body)
		switch r.URL.Path {
		case "/key/generate":
			require.NoError(t, json.Unmarshal(body, &generate))
			_, _ = w.Write([]byte(`{"key":"sk-fake-session-key","expires":"2030-01-01T00:00:00Z"}`))
		case "/key/list":
			require.Equal(t, "yari-session-s1", r.URL.Query().Get("key_alias"))
			_, _ = w.Write([]byte(`{"keys":[{"key_alias":"other","spend":9},{"key_alias":"yari-session-s1","spend":0.42}]}`))
		case "/key/delete":
			require.NoError(t, json.Unmarshal(body, &deleted))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &gateway.Client{BaseURL: srv.URL, MasterKey: "master-fake"}
	ctx := context.Background()
	key, err := c.CreateSessionKey(ctx, "s1", "claude-sonnet-5-5", 2, 30*time.Minute)
	require.NoError(t, err)
	require.Equal(t, "sk-fake-session-key", key.Key)
	require.Equal(t, "yari-session-s1", key.Alias)
	require.NotContains(t, key.String(), "sk-fake", "the key never prints")

	require.Equal(t, "yari-session-s1", generate["key_alias"])
	require.Equal(t, []any{"claude-sonnet-5-5"}, generate["models"], "restricted to exactly one model")
	require.EqualValues(t, 2, generate["max_budget"])
	require.Equal(t, "1800s", generate["duration"])

	spend, err := c.Spend(ctx, key.Alias)
	require.NoError(t, err)
	require.InDelta(t, 0.42, spend, 1e-9)

	require.NoError(t, c.Revoke(ctx, key.Alias))
	require.Equal(t, []any{"yari-session-s1"}, deleted["key_aliases"])
}

func TestErrorsNeverContainTheMasterKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad credentials: "+r.Header.Get("Authorization"), http.StatusUnauthorized)
	}))
	defer srv.Close()
	c := &gateway.Client{BaseURL: srv.URL, MasterKey: "master-fake"}
	_, err := c.CreateSessionKey(context.Background(), "s1", "m", 1, time.Minute)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "master-fake")
}

func TestBudgetExhaustedDetection(t *testing.T) {
	require.True(t, gateway.IsBudgetExhausted("litellm.BudgetExceededError: ExceededBudget: Crossed spend within team"))
	require.True(t, gateway.IsBudgetExhausted("Budget has been exceeded! Current cost: 2.1, Max budget: 2.0"))
	require.False(t, gateway.IsBudgetExhausted("connection reset by peer"))
}

func TestModelsFromTheRealConfigFile(t *testing.T) {
	path := filepath.Join("..", "..", "agent", "litellm", "config.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Skip("config not found")
	}
	m := &gateway.ConfigModels{Path: path}
	models, err := m.Models(context.Background())
	require.NoError(t, err)
	var aliases []string
	for _, x := range models {
		aliases = append(aliases, x.Alias)
	}
	require.Contains(t, aliases, "claude-sonnet-5-5")
	require.Contains(t, aliases, "gpt-6.1-sol")
	require.NotContains(t, aliases, "local-qwen", "the commented Ollama example is not a model")
	again, err := m.Models(context.Background())
	require.NoError(t, err)
	require.Equal(t, models, again)
}

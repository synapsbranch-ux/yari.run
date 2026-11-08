// Package config loads service configuration from the environment.
//
// Nothing here has a secret default. Local development values come from .env (see .env.example).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the configuration shared by the yari.run services. Each service reads the
// subset it needs and validates it with the Require* helpers.
type Config struct {
	Env         string // dev, staging, prod
	APIAddr     string
	CORSOrigins []string

	DatabaseURL           string // application role (yari_app), RLS applies
	MigrationsDatabaseURL string // owner role, used only by cmd/migrate

	ZitadelIssuer   string
	ZitadelClientID string

	AgentRuntime       string // "fake" or "openhands"
	AgentImage         string
	AgentDefaultModel  string
	AgentSessionBudget float64 // USD per Agent Session
	AgentMaxIterations int
	AgentSessionTTL    time.Duration
	SandboxRuntime     string // OCI runtime for sandboxes, e.g. runsc

	MCPURL  string // MCP URL as seen from inside the sandbox network
	MCPAddr string // listen address of cmd/mcp
	APIURL  string // Connect API base URL used by cmd/mcp and the CLI

	LiteLLMURL       string
	LiteLLMMasterKey string // worker side only, never forwarded anywhere
	// LLMBaseURL is the LiteLLM URL as seen from inside the sandbox network.
	LLMBaseURL string
}

// FromEnv reads the configuration from environment variables.
func FromEnv() Config {
	budget, err := strconv.ParseFloat(getenv("YARI_AGENT_SESSION_BUDGET_USD", "2"), 64)
	if err != nil {
		budget = 2
	}
	return Config{
		Env:         getenv("YARI_ENV", "dev"),
		APIAddr:     getenv("YARI_API_ADDR", ":8080"),
		CORSOrigins: splitCSV(os.Getenv("YARI_CORS_ORIGINS")),

		DatabaseURL:           os.Getenv("DATABASE_URL"),
		MigrationsDatabaseURL: os.Getenv("MIGRATIONS_DATABASE_URL"),

		ZitadelIssuer:   os.Getenv("ZITADEL_ISSUER"),
		ZitadelClientID: os.Getenv("ZITADEL_CLIENT_ID"),

		AgentRuntime:       getenv("YARI_AGENT_RUNTIME", "fake"),
		AgentImage:         getenv("YARI_AGENT_IMAGE", "yari-agent:dev"),
		AgentDefaultModel:  getenv("YARI_AGENT_DEFAULT_MODEL", "claude-sonnet-5-5"),
		AgentSessionBudget: budget,
		AgentMaxIterations: getenvInt("YARI_AGENT_MAX_ITERATIONS", 50),
		AgentSessionTTL:    time.Duration(getenvInt("YARI_AGENT_SESSION_TTL_MINUTES", 30)) * time.Minute,
		SandboxRuntime:     getenv("YARI_SANDBOX_RUNTIME", "runsc"),

		MCPURL:  getenv("YARI_MCP_URL", "http://yari-mcp:8090/mcp"),
		MCPAddr: getenv("YARI_MCP_ADDR", ":8090"),
		APIURL:  getenv("YARI_API_URL", "http://localhost:8080"),

		LiteLLMURL:       getenv("LITELLM_URL", "http://localhost:4000"),
		LiteLLMMasterKey: os.Getenv("LITELLM_MASTER_KEY"),
		LLMBaseURL:       getenv("YARI_LLM_BASE_URL", "http://litellm:4000"),
	}
}

// IsDev reports whether the process runs in the local development environment.
func (c Config) IsDev() bool { return c.Env == "dev" }

// RequireDatabase returns an error if the application database URL is missing.
func (c Config) RequireDatabase() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	return nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return def
	}
	return v
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

package sandbox

import (
	"fmt"
	"regexp"
	"strings"
)

// AgentServerKeyEnv is the exact name of the environment variable the OpenHands agent server reads
// its session API key from. It is the only secret allowed in a Sandbox environment.
const AgentServerKeyEnv = "OH_SESSION_API_KEYS_0"

// SessionKeyPrefix starts every agent server session key that yari.run generates, so a real key
// can never be mistaken for one.
const SessionKeyPrefix = "ohs_"

// forbiddenNameParts are name fragments that mark a credential. Matched case-insensitively.
var forbiddenNameParts = []string{
	"ANTHROPIC", "OPENAI", "AWS_", "DATABASE_URL", "MASTER_KEY", "PASSWORD",
	"SECRET", "TOKEN", "API_KEY", "APIKEY", "PRIVATE_KEY", "CREDENTIAL", "LITELLM",
}

// forbiddenValue matches values that look like long-lived credentials.
var forbiddenValue = regexp.MustCompile(`(?i)^(sk-[a-z0-9_-]{8,}|sk-ant-|AKIA[0-9A-Z]{8,}|ASIA[0-9A-Z]{8,}|ghp_|gho_|ghs_|github_pat_|glpat-|xox[abprs]-|AIza[0-9A-Za-z_-]{8,}|yari_|eyJ[a-z0-9_-]{8,}\.|-----BEGIN |postgres(ql)?://[^/\s]*@|mysql://[^/\s]*@|mongodb(\+srv)?://[^/\s]*@|redis://[^/\s]*@)`)

// ValidateEnv rejects any variable that looks like a long-lived credential. The only secret
// allowed is the agent server session key, under exactly AgentServerKeyEnv, and its value must
// carry SessionKeyPrefix. The LiteLLM virtual key and the yari session token are sent through the
// agent server API when the conversation is created, never as container environment.
func ValidateEnv(env map[string]string) error {
	for name, value := range env {
		if name == "" || strings.ContainsAny(name, "= \t\n") {
			return fmt.Errorf("sandbox: invalid environment variable name %q", name)
		}
		if name == AgentServerKeyEnv {
			if !strings.HasPrefix(value, SessionKeyPrefix) || len(value) < len(SessionKeyPrefix)+16 {
				return fmt.Errorf("sandbox: %s must be a generated session key (prefix %s)", AgentServerKeyEnv, SessionKeyPrefix)
			}
			continue
		}
		upper := strings.ToUpper(name)
		for _, bad := range forbiddenNameParts {
			if strings.Contains(upper, bad) {
				return fmt.Errorf("sandbox: environment variable %s looks like a credential and is not allowed in a sandbox", name)
			}
		}
		if forbiddenValue.MatchString(strings.TrimSpace(value)) {
			return fmt.Errorf("sandbox: the value of environment variable %s looks like a credential and is not allowed in a sandbox", name)
		}
	}
	return nil
}

package agent

import (
	"regexp"
	"strings"
)

// keyPatterns match secrets that should never be stored even if they were not given to us.
var keyPatterns = []*regexp.Regexp{
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{8,}`),
	regexp.MustCompile(`yari_[A-Za-z0-9_-]{8,}`),
	regexp.MustCompile(`ohs_[A-Za-z0-9_-]{8,}`),
	regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{12,}\b`),
	regexp.MustCompile(`\b(ghp|gho|ghs|ghu)_[A-Za-z0-9]{20,}\b`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`xox[abprs]-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`AIza[0-9A-Za-z_-]{20,}`),
	regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]{16,}`),
	regexp.MustCompile(`(?i)(postgres(?:ql)?|mysql|mongodb(?:\+srv)?|redis)://[^\s:/@"]+:[^\s@"]+@`),
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
}

// Redactor removes secrets from text and JSON before anything is stored or shown.
type Redactor struct{ secrets []string }

// NewRedactor knows the given secret values (the three session secrets). Empty and very short
// values are ignored so that redaction cannot mangle ordinary text.
func NewRedactor(secrets ...string) *Redactor {
	r := &Redactor{}
	for _, s := range secrets {
		if len(s) >= 8 {
			r.secrets = append(r.secrets, s)
		}
	}
	return r
}

// Redacted is what replaces a secret.
const Redacted = "[redacted]"

// String redacts known secrets and anything shaped like a credential.
func (r *Redactor) String(s string) string {
	for _, sec := range r.secrets {
		s = strings.ReplaceAll(s, sec, Redacted)
	}
	for _, p := range keyPatterns {
		s = p.ReplaceAllStringFunc(s, func(m string) string {
			if lower := strings.ToLower(m); strings.HasPrefix(lower, "bearer") {
				return m[:len("bearer ")] + Redacted
			}
			if i := strings.Index(m, "://"); i > 0 {
				return m[:i+3] + Redacted + "@"
			}
			return Redacted
		})
	}
	return s
}

// Bytes redacts a JSON document. Secrets are replaced inside the encoded text, which keeps the
// document valid JSON because the replacement contains no characters that need escaping.
func (r *Redactor) Bytes(b []byte) []byte { return []byte(r.String(string(b))) }

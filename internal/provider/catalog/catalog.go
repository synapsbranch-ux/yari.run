// Package catalog loads and validates the Provider Catalog: the list of clouds yari.run can
// deploy to, what each can run, and how far its adapter has come.
package catalog

import (
	_ "embed"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/synapsbranch-ux/yari.run/internal/provider"
)

//go:embed catalog.yaml
var embedded []byte

// Statuses.
const (
	StatusAlpha   = "alpha"
	StatusPlanned = "planned"
)

// Entry is one provider of the catalog.
type Entry struct {
	ID       string   `yaml:"id" json:"id"`
	Name     string   `yaml:"name" json:"name"`
	Category string   `yaml:"category" json:"category"`
	Kinds    []string `yaml:"kinds" json:"kinds"`
	Auth     []string `yaml:"auth" json:"auth"`
	Adapter  string   `yaml:"adapter" json:"adapter"`
	Status   string   `yaml:"status" json:"status"`
	Docs     string   `yaml:"docs" json:"docs"`
}

// Supports reports whether the entry can run the kind.
func (e Entry) Supports(k provider.Kind) bool {
	for _, v := range e.Kinds {
		if v == string(k) {
			return true
		}
	}
	return false
}

var (
	// Categories is the closed list of categories.
	Categories = []string{"paas", "cloud", "database", "storage", "cdn_dns", "email", "messaging", "search", "observability", "auth", "source"}
	// AuthMethods is the closed list of authentication methods.
	AuthMethods = []string{"oauth", "api_token", "api_key", "access_key", "iam_role", "workload_identity"}
	// Adapters is the closed list of adapter families.
	Adapters = []string{"native", "s3compat", "pulumi", "none"}
	// Statuses is the closed list of statuses.
	Statuses = []string{StatusAlpha, StatusPlanned}
)

// AlphaCount is the number of providers with a first adapter in this scaffold.
const AlphaCount = 3

// Catalog is an immutable, validated list of entries in file order.
type Catalog struct {
	entries []Entry
	byID    map[string]int
}

// Load parses and validates the embedded catalog.yaml.
func Load() (*Catalog, error) { return Parse(embedded) }

// MustLoad is Load for process start-up: a broken embedded catalog is a build defect.
func MustLoad() *Catalog {
	c, err := Load()
	if err != nil {
		panic(err)
	}
	return c
}

// Parse parses and validates catalog YAML.
func Parse(data []byte) (*Catalog, error) {
	var entries []Entry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("catalog: parse: %w", err)
	}
	if err := Validate(entries); err != nil {
		return nil, err
	}
	c := &Catalog{entries: entries, byID: make(map[string]int, len(entries))}
	for i, e := range entries {
		c.byID[e.ID] = i
	}
	return c, nil
}

// Validate checks the catalog rules: unique ids, known categories, kinds, auth methods, adapter
// families and statuses, https docs URLs, and exactly AlphaCount alpha entries.
func Validate(entries []Entry) error {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	seen := map[string]bool{}
	alpha := 0
	for i, e := range entries {
		where := fmt.Sprintf("entry %d (%q)", i, e.ID)
		if e.ID == "" || strings.ToLower(e.ID) != e.ID || strings.ContainsAny(e.ID, " _/") {
			add("%s: id must be non-empty lowercase kebab-case", where)
		}
		if seen[e.ID] {
			add("%s: duplicate id", where)
		}
		seen[e.ID] = true
		if strings.TrimSpace(e.Name) == "" {
			add("%s: name is required", where)
		}
		if !contains(Categories, e.Category) {
			add("%s: unknown category %q", where, e.Category)
		}
		if len(e.Kinds) == 0 {
			add("%s: at least one kind is required", where)
		}
		kindSeen := map[string]bool{}
		for _, k := range e.Kinds {
			if !provider.Kind(k).Valid() {
				add("%s: unknown kind %q", where, k)
			}
			if kindSeen[k] {
				add("%s: duplicate kind %q", where, k)
			}
			kindSeen[k] = true
		}
		if len(e.Auth) == 0 {
			add("%s: at least one auth method is required", where)
		}
		for _, a := range e.Auth {
			if !contains(AuthMethods, a) {
				add("%s: unknown auth method %q", where, a)
			}
		}
		if !contains(Adapters, e.Adapter) {
			add("%s: unknown adapter family %q", where, e.Adapter)
		}
		if !contains(Statuses, e.Status) {
			add("%s: unknown status %q", where, e.Status)
		}
		if e.Status == StatusAlpha {
			alpha++
		}
		if u, err := url.Parse(e.Docs); err != nil || u.Scheme != "https" || u.Host == "" {
			add("%s: docs must be an https URL, got %q", where, e.Docs)
		}
	}
	if alpha != AlphaCount {
		add("exactly %d entries must be alpha, found %d", AlphaCount, alpha)
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("catalog: invalid:\n  %s", strings.Join(problems, "\n  "))
	}
	return nil
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// Entries returns a copy of all entries in file order.
func (c *Catalog) Entries() []Entry { return append([]Entry(nil), c.entries...) }

// Get returns the entry for an id.
func (c *Catalog) Get(id string) (Entry, bool) {
	i, ok := c.byID[id]
	if !ok {
		return Entry{}, false
	}
	return c.entries[i], true
}

// Filter returns entries matching an optional category and an optional kind.
func (c *Catalog) Filter(category string, kind provider.Kind) []Entry {
	var out []Entry
	for _, e := range c.entries {
		if category != "" && e.Category != category {
			continue
		}
		if kind != "" && !e.Supports(kind) {
			continue
		}
		out = append(out, e)
	}
	return out
}

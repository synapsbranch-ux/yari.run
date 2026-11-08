// Package provider defines the interface every cloud adapter implements, and the shared types
// the orchestrator and the API use. yari.run never hosts workloads: adapters drive the native API
// of a cloud the user already pays for, using short-lived leases from the credential broker.
package provider

import (
	"context"
	"errors"
	"time"
)

// Kind is what a service is. Keep this list in sync with the catalog validator.
type Kind string

const (
	KindWeb           Kind = "web"
	KindWorker        Kind = "worker"
	KindCron          Kind = "cron"
	KindFunction      Kind = "function"
	KindStatic        Kind = "static"
	KindPostgres      Kind = "postgres"
	KindMySQL         Kind = "mysql"
	KindSQLite        Kind = "sqlite"
	KindMongoDB       Kind = "mongodb"
	KindRedis         Kind = "redis"
	KindKV            Kind = "kv"
	KindBucket        Kind = "bucket"
	KindQueue         Kind = "queue"
	KindSearch        Kind = "search"
	KindEmail         Kind = "email"
	KindDNS           Kind = "dns"
	KindCDN           Kind = "cdn"
	KindDomain        Kind = "domain"
	KindVM            Kind = "vm"
	KindObservability Kind = "observability"
	KindAuth          Kind = "auth"
	KindRepository    Kind = "repository"
	KindRegistry      Kind = "registry"
)

// AllKinds is the closed list of kinds.
var AllKinds = []Kind{
	KindWeb, KindWorker, KindCron, KindFunction, KindStatic, KindPostgres, KindMySQL, KindSQLite,
	KindMongoDB, KindRedis, KindKV, KindBucket, KindQueue, KindSearch, KindEmail, KindDNS, KindCDN,
	KindDomain, KindVM, KindObservability, KindAuth, KindRepository, KindRegistry,
}

// Valid reports whether k is a known kind.
func (k Kind) Valid() bool {
	for _, v := range AllKinds {
		if v == k {
			return true
		}
	}
	return false
}

// IsDataStore reports whether the kind holds user data. Deleting one is never allowed for agents.
func (k Kind) IsDataStore() bool {
	switch k {
	case KindPostgres, KindMySQL, KindMongoDB, KindSQLite, KindRedis, KindBucket:
		return true
	}
	return false
}

// CapabilitySet is the set of kinds an adapter can manage.
type CapabilitySet map[Kind]bool

// Lease is a short-lived, operation-scoped credential issued by the broker.
type Lease interface {
	Token(ctx context.Context) (string, error)
	Scope() string
}

// OutputRef points at an output of another service in the same plan.
type OutputRef struct{ Service, Key string }

// EnvValue is a literal value or a reference to another service's output.
type EnvValue struct {
	Value      string
	FromOutput *OutputRef
	Secret     bool
}

// ServiceSpec is the desired state of one Service.
type ServiceSpec struct {
	Name     string
	Kind     Kind
	Provider string // catalog id
	Config   map[string]string
	Env      map[string]EnvValue
}

// Output is a value a provider returns after an apply.
type Output struct {
	Value  string
	Secret bool // String() and logs must redact
}

// String redacts secret values so an Output is safe to log.
func (o Output) String() string {
	if o.Secret {
		return "[redacted]"
	}
	return o.Value
}

// GoString keeps %#v from leaking a secret.
func (o Output) GoString() string { return "provider.Output{" + o.String() + "}" }

// Outputs are the named outputs of one service.
type Outputs map[string]Output

// Action is what a Change does.
type Action string

// Change actions.
const (
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
	ActionNoop   Action = "noop"
)

// Risk grades a change.
type Risk string

// Risk levels.
const (
	RiskLow    Risk = "low"
	RiskMedium Risk = "medium"
	RiskHigh   Risk = "high"
)

// Change is one reviewable step of a Plan.
type Change struct {
	Service        string
	Action         Action
	After          *ServiceSpec
	Risk           Risk
	CostDeltaCents int64
	Description    string
}

// Ref identifies a resource at a provider.
type Ref struct {
	Provider   string
	ExternalID string
	Tags       map[string]string // always yari:workspace, yari:project, yari:service
}

// ObservedService is the current state of a service at a provider.
type ObservedService struct {
	Ref    Ref
	Status string
	Spec   *ServiceSpec
}

// LogQuery selects log lines.
type LogQuery struct {
	Since time.Time
	Until time.Time
	Limit int
	Query string
}

// LogLine is one line of service output.
type LogLine struct {
	At      time.Time
	Stream  string // stdout, stderr, system
	Message string
}

// MetricQuery selects a time series.
type MetricQuery struct {
	Name  string // cpu, memory, requests
	Since time.Time
	Until time.Time
	Step  time.Duration
}

// Point is one sample of a time series.
type Point struct {
	At    time.Time
	Value float64
}

// Provider is implemented once per catalog adapter family member.
type Provider interface {
	Name() string
	Capabilities() CapabilitySet
	Validate(ctx context.Context, lease Lease) error
	Plan(ctx context.Context, lease Lease, desired []ServiceSpec, observed []ObservedService) ([]Change, error)
	Apply(ctx context.Context, lease Lease, change Change) (Ref, Outputs, error)
	Observe(ctx context.Context, lease Lease, ref Ref) (ObservedService, error)
	Logs(ctx context.Context, lease Lease, ref Ref, q LogQuery) (<-chan LogLine, error)
	Metrics(ctx context.Context, lease Lease, ref Ref, q MetricQuery) ([]Point, error)
	Destroy(ctx context.Context, lease Lease, ref Ref) error
}

var (
	// ErrNotImplemented is returned by stub adapters.
	ErrNotImplemented = errors.New("provider: not implemented")
	// ErrNotSupported means the catalog lists the provider but no adapter exists yet.
	// Wrap it with the provider's docs URL.
	ErrNotSupported = errors.New("provider: not supported yet")
)

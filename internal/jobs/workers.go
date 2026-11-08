package jobs

import (
	"context"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/synapsbranch-ux/yari.run/internal/agent"
	"github.com/synapsbranch-ux/yari.run/internal/config"
	"github.com/synapsbranch-ux/yari.run/internal/orchestrator"
	"github.com/synapsbranch-ux/yari.run/internal/provider"
	"github.com/synapsbranch-ux/yari.run/internal/provider/fake"
	"github.com/synapsbranch-ux/yari.run/internal/sandbox"
)

// Deps are the collaborators of the workers. The agent fields are optional: without Sandboxes
// only the plan worker is registered.
type Deps struct {
	Pool     *pgxpool.Pool
	Registry *provider.Registry
	Leaser   orchestrator.Leaser

	Config     config.Config
	Sandboxes  sandbox.Manager
	Gateway    ModelGateway
	Runtime    agent.Runtime
	Live       *LiveSessions
	Prompt     string
	SampleRepo fs.FS
	SpendPoll  time.Duration
}

// NewWorkers registers every worker.
func NewWorkers(d Deps) (*river.Workers, error) {
	w := river.NewWorkers()
	if err := river.AddWorkerSafely(w, &ApplyPlanWorker{Pool: d.Pool, Registry: d.Registry, Leaser: d.Leaser}); err != nil {
		return nil, err
	}
	if d.Sandboxes != nil {
		if err := river.AddWorkerSafely(w, &SessionWorker{
			Pool: d.Pool, Config: d.Config, Sandboxes: d.Sandboxes, Gateway: d.Gateway, Runtime: d.Runtime,
			Live: d.Live, Prompt: d.Prompt, SampleRepo: d.SampleRepo, SpendPoll: d.SpendPoll,
		}); err != nil {
			return nil, err
		}
		if err := river.AddWorkerSafely(w, &ReapWorker{Pool: d.Pool, Sandboxes: d.Sandboxes, Gateway: d.Gateway, MaxAge: d.Config.AgentSessionTTL + 10*time.Minute}); err != nil {
			return nil, err
		}
	}
	return w, nil
}

// FakeLeaser issues fake leases. The credential broker will replace it.
type FakeLeaser struct{}

// Lease implements orchestrator.Leaser.
func (FakeLeaser) Lease(_ context.Context, providerID string) (provider.Lease, error) {
	return fake.Lease{ScopeName: providerID}, nil
}

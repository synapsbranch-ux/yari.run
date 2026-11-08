package jobs

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/synapsbranch-ux/yari.run/internal/orchestrator"
	"github.com/synapsbranch-ux/yari.run/internal/provider"
	"github.com/synapsbranch-ux/yari.run/internal/provider/fake"
)

// Deps are the collaborators of the workers.
type Deps struct {
	Pool     *pgxpool.Pool
	Registry *provider.Registry
	Leaser   orchestrator.Leaser
}

// NewWorkers registers every worker.
func NewWorkers(d Deps) (*river.Workers, error) {
	w := river.NewWorkers()
	if err := river.AddWorkerSafely(w, &ApplyPlanWorker{Pool: d.Pool, Registry: d.Registry, Leaser: d.Leaser}); err != nil {
		return nil, err
	}
	return w, nil
}

// FakeLeaser issues fake leases. The credential broker will replace it.
type FakeLeaser struct{}

// Lease implements orchestrator.Leaser.
func (FakeLeaser) Lease(_ context.Context, providerID string) (provider.Lease, error) {
	return fake.Lease{ScopeName: providerID}, nil
}

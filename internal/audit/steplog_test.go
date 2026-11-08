package audit_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/synapsbranch-ux/yari.run/internal/audit"
	"github.com/synapsbranch-ux/yari.run/internal/db"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbsqlc"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbtest"
)

func buildChain(t *testing.T, n int) []dbsqlc.AgentStep {
	t.Helper()
	var steps []dbsqlc.AgentStep
	prev := []byte{}
	for i := 1; i <= n; i++ {
		raw, err := audit.Canonical([]byte(`{"b":2,"a":[1,2,3],"i":` + string(rune('0'+i)) + `}`))
		require.NoError(t, err)
		h := audit.ComputeHash(prev, int32(i), "action", "terminal", "step", raw)
		steps = append(steps, dbsqlc.AgentStep{Seq: int32(i), Kind: "action", Tool: pgtype.Text{String: "terminal", Valid: true}, Summary: "step", RawEvent: raw, PrevHash: prev, Hash: h})
		prev = h
	}
	return steps
}

func TestVerifyChainAcceptsValidChain(t *testing.T) {
	require.NoError(t, audit.VerifyChain(buildChain(t, 5)))
	require.NoError(t, audit.VerifyChain(nil))
}

func TestVerifyChainDetectsTampering(t *testing.T) {
	cases := map[string]func([]dbsqlc.AgentStep) []dbsqlc.AgentStep{
		"modified summary": func(s []dbsqlc.AgentStep) []dbsqlc.AgentStep { s[2].Summary = "edited"; return s },
		"modified raw":     func(s []dbsqlc.AgentStep) []dbsqlc.AgentStep { s[1].RawEvent = []byte(`{"a":[]}`); return s },
		"modified kind":    func(s []dbsqlc.AgentStep) []dbsqlc.AgentStep { s[0].Kind = "error"; return s },
		"removed middle":   func(s []dbsqlc.AgentStep) []dbsqlc.AgentStep { return append(s[:2], s[3:]...) },
		"removed first":    func(s []dbsqlc.AgentStep) []dbsqlc.AgentStep { return s[1:] },
		"reordered":        func(s []dbsqlc.AgentStep) []dbsqlc.AgentStep { s[1], s[2] = s[2], s[1]; return s },
		"forged hash":      func(s []dbsqlc.AgentStep) []dbsqlc.AgentStep { s[3].Hash = make([]byte, 32); return s },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			err := audit.VerifyChain(mutate(buildChain(t, 5)))
			require.ErrorIs(t, err, audit.ErrTampered)
		})
	}
}

func TestHashFieldBoundariesAreUnambiguous(t *testing.T) {
	a := audit.ComputeHash(nil, 1, "ab", "c", "d", []byte("{}"))
	b := audit.ComputeHash(nil, 1, "a", "bc", "d", []byte("{}"))
	require.NotEqual(t, a, b)
}

func TestCanonicalIsStable(t *testing.T) {
	x, err := audit.Canonical([]byte(`{"b": 1, "a": {"d":[1, 2.5], "c": "x"}}`))
	require.NoError(t, err)
	y, err := audit.Canonical(x)
	require.NoError(t, err)
	require.Equal(t, string(x), string(y))
	require.Equal(t, `{"a":{"c":"x","d":[1,2.5]},"b":1}`, string(x))
}

func TestAppendStepPersistsVerifiableChain(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	o := dbtest.SeedOrg(t, pool)
	sessionID := db.NewID()

	require.NoError(t, db.WithTenant(ctx, pool, o.ID, func(ctx context.Context, tx pgx.Tx) error {
		q := dbsqlc.New(tx)
		agent, err := q.CreatePrincipal(ctx, dbsqlc.CreatePrincipalParams{ID: db.NewID(), OrganizationID: o.ID, Kind: "agent", DisplayName: "yari-agent/test"})
		require.NoError(t, err)
		_, err = q.CreateAgentSession(ctx, dbsqlc.CreateAgentSessionParams{ID: sessionID, OrganizationID: o.ID, EnvironmentID: o.ProdEnvID, AgentPrincipalID: agent.ID, StartedBy: o.PrincipalID, Goal: "g", Runtime: "fake", Model: "m", BudgetUsdMicros: 1, MaxIterations: 1})
		require.NoError(t, err)
		for i, kind := range []string{"message", "action", "observation", "guardrail"} {
			_, err := audit.AppendStep(ctx, q, o.ID, sessionID, audit.StepInput{Kind: kind, Tool: "terminal", Location: "sandbox", Summary: "s", RawEvent: []byte(`{"z":1,"y":{"k":[true,null]},"n":0.5}`), PlanID: uuid.Nil})
			require.NoError(t, err, "step %d", i)
		}
		return nil
	}))

	// Read back through a fresh transaction: jsonb normalization must not break verification.
	require.NoError(t, db.WithTenant(ctx, pool, o.ID, func(ctx context.Context, tx pgx.Tx) error {
		steps, err := dbsqlc.New(tx).ListAgentSteps(ctx, dbsqlc.ListAgentStepsParams{SessionID: sessionID, AfterSeq: 0})
		require.NoError(t, err)
		require.Len(t, steps, 4)
		require.NoError(t, audit.VerifyChain(steps))
		steps[1].Summary = "tampered"
		require.ErrorIs(t, audit.VerifyChain(steps), audit.ErrTampered)
		return nil
	}))
}

// Package jobs defines the River jobs of yari.run and their workers.
package jobs

import (
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// ApplyPlanArgs applies an approved or directly applicable Plan.
type ApplyPlanArgs struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	PlanID         uuid.UUID `json:"plan_id"`
}

// Kind implements river.JobArgs.
func (ApplyPlanArgs) Kind() string { return "apply_plan" }

// InsertOpts: an apply is never retried automatically, because a half-applied plan needs a person.
func (ApplyPlanArgs) InsertOpts() river.InsertOpts { return river.InsertOpts{MaxAttempts: 1} }

// RunAgentSessionArgs runs one Agent Session.
//
// MCPToken is the session-scoped yari API token the agent uses over MCP. It is created in the
// same transaction as the job, expires with the session and is revoked when the session ends.
// It is never returned to API callers and never logged. Job rows keep their args after the job
// finishes, which is acceptable only because the token is already dead by then.
type RunAgentSessionArgs struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	SessionID      uuid.UUID `json:"session_id"`
	MCPToken       string    `json:"mcp_token"`
}

// Kind implements river.JobArgs.
func (RunAgentSessionArgs) Kind() string { return "run_agent_session" }

// InsertOpts: a session is never retried, since a retry would repeat actions of an agent.
func (RunAgentSessionArgs) InsertOpts() river.InsertOpts { return river.InsertOpts{MaxAttempts: 1} }

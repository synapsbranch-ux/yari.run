// Package plan converts between the API representation of a Plan's changes, the stored JSON and
// the provider types, and grades changes for risk and cost.
package plan

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"

	yariv1 "github.com/synapsbranch-ux/yari.run/gen/yari/v1"
	"github.com/synapsbranch-ux/yari.run/internal/provider"
)

var actionToProto = map[provider.Action]yariv1.ChangeAction{
	provider.ActionCreate: yariv1.ChangeAction_CHANGE_ACTION_CREATE,
	provider.ActionUpdate: yariv1.ChangeAction_CHANGE_ACTION_UPDATE,
	provider.ActionDelete: yariv1.ChangeAction_CHANGE_ACTION_DELETE,
	provider.ActionNoop:   yariv1.ChangeAction_CHANGE_ACTION_NOOP,
}

var riskToProto = map[provider.Risk]yariv1.Risk{
	provider.RiskLow:    yariv1.Risk_RISK_LOW,
	provider.RiskMedium: yariv1.Risk_RISK_MEDIUM,
	provider.RiskHigh:   yariv1.Risk_RISK_HIGH,
}

// RiskToProto converts a risk.
func RiskToProto(r provider.Risk) yariv1.Risk { return riskToProto[r] }

// RiskFromProto converts a risk, defaulting to low.
func RiskFromProto(r yariv1.Risk) provider.Risk {
	for k, v := range riskToProto {
		if v == r {
			return k
		}
	}
	return provider.RiskLow
}

// ActionFromProto converts an action. The second result is false for unspecified or unknown values.
func ActionFromProto(a yariv1.ChangeAction) (provider.Action, bool) {
	for k, v := range actionToProto {
		if v == a {
			return k, true
		}
	}
	return "", false
}

// SpecFromProto converts a service spec.
func SpecFromProto(s *yariv1.ServiceSpec) *provider.ServiceSpec {
	if s == nil {
		return nil
	}
	out := &provider.ServiceSpec{Name: s.Name, Kind: provider.Kind(s.Kind), Provider: s.Provider, Config: s.Config, Env: map[string]provider.EnvValue{}}
	for k, v := range s.Env {
		ev := provider.EnvValue{Value: v.Value, Secret: v.Secret}
		if v.FromOutput != nil {
			ev.FromOutput = &provider.OutputRef{Service: v.FromOutput.Service, Key: v.FromOutput.Key}
		}
		out.Env[k] = ev
	}
	return out
}

// SpecToProto converts a service spec.
func SpecToProto(s *provider.ServiceSpec) *yariv1.ServiceSpec {
	if s == nil {
		return nil
	}
	out := &yariv1.ServiceSpec{Name: s.Name, Kind: string(s.Kind), Provider: s.Provider, Config: s.Config, Env: map[string]*yariv1.EnvValue{}}
	for k, v := range s.Env {
		ev := &yariv1.EnvValue{Value: v.Value, Secret: v.Secret}
		if v.FromOutput != nil {
			ev.FromOutput = &yariv1.OutputRef{Service: v.FromOutput.Service, Key: v.FromOutput.Key}
		}
		out.Env[k] = ev
	}
	return out
}

// FromProto converts a plan change.
func FromProto(c *yariv1.PlanChange) (provider.Change, error) {
	action, ok := ActionFromProto(c.Action)
	if !ok {
		return provider.Change{}, fmt.Errorf("change %q: unknown action", c.Service)
	}
	return provider.Change{
		Service: c.Service, Action: action, After: SpecFromProto(c.After),
		Risk: RiskFromProto(c.Risk), CostDeltaCents: c.CostDeltaCents, Description: c.Description,
	}, nil
}

// ToProto converts a plan change.
func ToProto(c provider.Change) *yariv1.PlanChange {
	return &yariv1.PlanChange{
		Service: c.Service, Action: actionToProto[c.Action], After: SpecToProto(c.After),
		Risk: riskToProto[c.Risk], CostDeltaCents: c.CostDeltaCents, Description: c.Description,
	}
}

// Marshal stores changes as a JSON array of protojson objects.
func Marshal(changes []*yariv1.PlanChange) ([]byte, error) {
	raw := make([]json.RawMessage, 0, len(changes))
	for _, c := range changes {
		b, err := protojson.Marshal(c)
		if err != nil {
			return nil, err
		}
		raw = append(raw, b)
	}
	return json.Marshal(raw)
}

// Unmarshal reads what Marshal wrote.
func Unmarshal(data []byte) ([]*yariv1.PlanChange, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	out := make([]*yariv1.PlanChange, 0, len(raw))
	for _, r := range raw {
		c := &yariv1.PlanChange{}
		if err := protojson.Unmarshal(r, c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// Providers converts stored changes to provider changes.
func Providers(changes []*yariv1.PlanChange) ([]provider.Change, error) {
	out := make([]provider.Change, 0, len(changes))
	for _, c := range changes {
		pc, err := FromProto(c)
		if err != nil {
			return nil, err
		}
		out = append(out, pc)
	}
	return out, nil
}

// monthlyCents is a rough, fixed price model used only to give plans a cost delta in this
// scaffold. Real adapters will report provider pricing.
var monthlyCents = map[provider.Kind]int64{
	provider.KindWeb: 500, provider.KindWorker: 500, provider.KindCron: 100, provider.KindVM: 1500,
	provider.KindPostgres: 2500, provider.KindMySQL: 2500, provider.KindMongoDB: 2500, provider.KindRedis: 1000,
	provider.KindSQLite: 0, provider.KindKV: 0, provider.KindBucket: 200, provider.KindQueue: 100,
	provider.KindSearch: 1500, provider.KindEmail: 0, provider.KindFunction: 0, provider.KindStatic: 0,
}

// Assess fills in risk and cost for a change from its action and kind.
//
// create: low risk, adds the monthly price. update: medium risk, no cost change.
// delete: high risk, removes the monthly price.
func Assess(action provider.Action, kind provider.Kind) (provider.Risk, int64) {
	price := monthlyCents[kind]
	switch action {
	case provider.ActionCreate:
		return provider.RiskLow, price
	case provider.ActionUpdate:
		if kind.IsDataStore() {
			return provider.RiskHigh, 0
		}
		return provider.RiskMedium, 0
	case provider.ActionDelete:
		return provider.RiskHigh, -price
	}
	return provider.RiskLow, 0
}

// Summarize returns the total cost delta and the highest risk of a set of changes.
func Summarize(changes []provider.Change) (int64, provider.Risk) {
	var cost int64
	risk := provider.RiskLow
	rank := map[provider.Risk]int{provider.RiskLow: 0, provider.RiskMedium: 1, provider.RiskHigh: 2}
	for _, c := range changes {
		cost += c.CostDeltaCents
		if rank[c.Risk] > rank[risk] {
			risk = c.Risk
		}
	}
	return cost, risk
}

// RedactForResponse removes literal values of secret env entries before a plan is returned.
func RedactForResponse(changes []*yariv1.PlanChange) {
	for _, c := range changes {
		if c.After == nil {
			continue
		}
		for _, v := range c.After.Env {
			if v.Secret {
				v.Value = ""
			}
		}
	}
}

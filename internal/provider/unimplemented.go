package provider

import "context"

// Unimplemented implements every Provider method except Name and Capabilities by returning
// ErrNotImplemented. Adapter stubs embed it so they satisfy Provider until real code lands.
type Unimplemented struct{}

// Validate implements Provider.
func (Unimplemented) Validate(context.Context, Lease) error { return ErrNotImplemented }

// Plan implements Provider.
func (Unimplemented) Plan(context.Context, Lease, []ServiceSpec, []ObservedService) ([]Change, error) {
	return nil, ErrNotImplemented
}

// Apply implements Provider.
func (Unimplemented) Apply(context.Context, Lease, Change) (Ref, Outputs, error) {
	return Ref{}, nil, ErrNotImplemented
}

// Observe implements Provider.
func (Unimplemented) Observe(context.Context, Lease, Ref) (ObservedService, error) {
	return ObservedService{}, ErrNotImplemented
}

// Logs implements Provider.
func (Unimplemented) Logs(context.Context, Lease, Ref, LogQuery) (<-chan LogLine, error) {
	return nil, ErrNotImplemented
}

// Metrics implements Provider.
func (Unimplemented) Metrics(context.Context, Lease, Ref, MetricQuery) ([]Point, error) {
	return nil, ErrNotImplemented
}

// Destroy implements Provider.
func (Unimplemented) Destroy(context.Context, Lease, Ref) error { return ErrNotImplemented }

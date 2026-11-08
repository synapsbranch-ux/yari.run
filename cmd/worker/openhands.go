package main

import (
	"github.com/synapsbranch-ux/yari.run/internal/agent"
	"github.com/synapsbranch-ux/yari.run/internal/agent/openhands"
	"github.com/synapsbranch-ux/yari.run/internal/config"
)

// newOpenHandsRuntime returns the runtime that drives the OpenHands agent server in each sandbox.
func newOpenHandsRuntime(config.Config) (agent.Runtime, error) {
	return &openhands.Runtime{}, nil
}

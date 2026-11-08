package main

import (
	"errors"

	"github.com/synapsbranch-ux/yari.run/internal/agent"
	"github.com/synapsbranch-ux/yari.run/internal/config"
)

// newOpenHandsRuntime is replaced once the OpenHands runtime lands.
func newOpenHandsRuntime(config.Config) (agent.Runtime, error) {
	return nil, errors.New("the openhands runtime is not built into this binary yet")
}

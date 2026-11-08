// Package prompts embeds the operating rules given to the agent.
package prompts

import _ "embed"

// System is the system prompt of every Agent Session.
//
//go:embed system.md
var System string

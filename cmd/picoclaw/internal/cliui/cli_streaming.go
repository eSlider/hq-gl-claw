package cliui

import (
	"encoding/json"
	"fmt"

	"github.com/sipeed/picoclaw/pkg/config"
)

// EnableCLIAgentStreaming opts the CLI agent path into provider streaming:
// a synthetic disabled wecom-typed "cli" channel with streaming settings, plus
// streaming enabled on the default model entry.
func EnableCLIAgentStreaming(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("nil config")
	}
	if cfg.Channels == nil {
		cfg.Channels = config.ChannelsConfig{}
	}

	settings := config.WeComSettings{
		Streaming: config.StreamingConfig{
			Enabled:         true,
			ThrottleSeconds: 0,
			MinGrowthChars:  1,
		},
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	ch := &config.Channel{
		Type:     config.ChannelWeCom,
		Enabled:  false,
		Settings: config.RawNode(raw),
	}
	ch.SetName("cli")
	target := &config.WeComSettings{}
	if err := ch.Decode(target); err != nil {
		return fmt.Errorf("decode cli channel: %w", err)
	}
	cfg.Channels["cli"] = ch

	modelName := cfg.Agents.Defaults.ModelName
	for _, m := range cfg.ModelList {
		if m == nil {
			continue
		}
		if modelName == "" || m.ModelName == modelName {
			m.Streaming.Enabled = true
			if modelName != "" {
				break
			}
		}
	}
	return nil
}

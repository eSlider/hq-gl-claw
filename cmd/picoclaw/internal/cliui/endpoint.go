package cliui

import (
	"net/url"
	"strings"

	"github.com/sipeed/picoclaw/pkg/config"
)

// EndpointInfo is the active model + API shown in the TUI status bar.
type EndpointInfo struct {
	ModelName string // user-facing alias (model_name)
	Model     string // provider model id
	APIBase   string
}

// FormatEndpoint compactly renders "model · host:port" for the status bar.
func FormatEndpoint(ep EndpointInfo) string {
	name := strings.TrimSpace(ep.ModelName)
	if name == "" {
		name = strings.TrimSpace(ep.Model)
	}
	if name == "" {
		name = "?"
	}
	host := ShortAPIHost(ep.APIBase)
	if host == "" {
		return name
	}
	return name + " · " + host
}

// ShortAPIHost returns host[:port] from an api_base URL.
func ShortAPIHost(apiBase string) string {
	apiBase = strings.TrimSpace(apiBase)
	if apiBase == "" {
		return ""
	}
	if !strings.Contains(apiBase, "://") {
		apiBase = "http://" + apiBase
	}
	u, err := url.Parse(apiBase)
	if err != nil || u.Host == "" {
		// Fallback: strip path crudely.
		s := strings.TrimPrefix(strings.TrimPrefix(apiBase, "https://"), "http://")
		if i := strings.IndexByte(s, '/'); i >= 0 {
			s = s[:i]
		}
		return s
	}
	return u.Host
}

// EndpointFromConfig resolves the active model entry for status display.
func EndpointFromConfig(cfg *config.Config) EndpointInfo {
	if cfg == nil {
		return EndpointInfo{}
	}
	name := cfg.Agents.Defaults.GetModelName()
	ep := EndpointInfo{ModelName: name}
	mc, err := cfg.GetModelConfig(name)
	if err != nil || mc == nil {
		return ep
	}
	ep.ModelName = mc.ModelName
	ep.Model = mc.Model
	ep.APIBase = mc.APIBase
	if ep.APIBase == "" {
		ep.APIBase = providersResolveAPIBase(mc)
	}
	return ep
}

// providersResolveAPIBase is a thin wrapper so cliui does not import providers
// cycles; duplicates ResolveAPIBase for display when APIBase is set (common case).
func providersResolveAPIBase(mc *config.ModelConfig) string {
	if mc == nil {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(mc.APIBase), "/")
}

// ListEnabledModelNames returns enabled model_name entries in config order.
func ListEnabledModelNames(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	out := make([]string, 0, len(cfg.ModelList))
	for _, m := range cfg.ModelList {
		if m == nil || !m.Enabled || strings.TrimSpace(m.ModelName) == "" {
			continue
		}
		out = append(out, m.ModelName)
	}
	return out
}

// NextModelName returns the next enabled model after current (wraps).
func NextModelName(cfg *config.Config, current string) string {
	names := ListEnabledModelNames(cfg)
	if len(names) == 0 {
		return current
	}
	current = strings.TrimSpace(current)
	idx := -1
	for i, n := range names {
		if n == current {
			idx = i
			break
		}
	}
	return names[(idx+1)%len(names)]
}

// EnsureBonsaiLocal inserts/updates the local llama.cpp OpenAI-compatible
// endpoint (127.0.0.1:9988) and optionally makes it the default model.
func EnsureBonsaiLocal(cfg *config.Config, setDefault bool) {
	if cfg == nil {
		return
	}
	const (
		name    = "bonsai"
		apiBase = "http://127.0.0.1:9988/v1"
		modelID = "bonsai"
	)
	found := false
	for _, m := range cfg.ModelList {
		if m == nil {
			continue
		}
		if m.ModelName == name || strings.Contains(m.APIBase, ":9988") {
			found = true
			m.Enabled = true
			if m.ModelName == name {
				if m.APIBase == "" {
					m.APIBase = apiBase
				}
				if m.Model == "" {
					m.Model = modelID
				}
				if m.Provider == "" {
					m.Provider = "openai"
				}
				m.Streaming.Enabled = true
			}
		}
	}
	if !found {
		cfg.ModelList = append(cfg.ModelList, &config.ModelConfig{
			ModelName: name,
			Provider:  "openai",
			Model:     modelID,
			APIBase:   apiBase,
			Enabled:   true,
			Streaming: config.ModelStreamingConfig{Enabled: true},
		})
	}
	if setDefault {
		cfg.Agents.Defaults.ModelName = name
	}
}

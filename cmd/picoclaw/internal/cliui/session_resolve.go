package cliui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// DefaultCLISession is used when no prior cli session exists.
	DefaultCLISession = "cli:default"
	lastCLISessionFile = "last_cli_session"
)

// ResolveCLISessionOpts configures interactive/default session selection.
type ResolveCLISessionOpts struct {
	Explicit    string // value from --session
	ExplicitSet bool   // true when user passed -s/--session
	Keys        []string
	Last        string // persisted last session key
	Fallback    string // when nothing else matches (default cli:default)
}

// ResolveCLISession picks which session to open.
// Explicit -s always wins; otherwise restore Last if still known, else most
// recent cli:* key, else Fallback.
func ResolveCLISession(opts ResolveCLISessionOpts) string {
	if opts.Fallback == "" {
		opts.Fallback = DefaultCLISession
	}
	if opts.ExplicitSet {
		if strings.TrimSpace(opts.Explicit) == "" {
			return opts.Fallback
		}
		return strings.TrimSpace(opts.Explicit)
	}
	last := strings.TrimSpace(opts.Last)
	if last != "" && containsKey(opts.Keys, last) {
		return last
	}
	if recent := MostRecentCLISession(opts.Keys); recent != "" {
		return recent
	}
	if last != "" {
		return last
	}
	return opts.Fallback
}

// MostRecentCLISession returns the newest cli:<unixnano> key, or "" .
// Non-numeric cli keys (e.g. cli:default) lose to numeric ones; if only
// non-numeric cli keys exist, returns the lexicographically greatest.
func MostRecentCLISession(keys []string) string {
	var bestNum string
	var bestVal int64 = -1
	var bestOther string
	for _, k := range keys {
		if !strings.HasPrefix(k, "cli:") {
			continue
		}
		suffix := strings.TrimPrefix(k, "cli:")
		if n, err := strconv.ParseInt(suffix, 10, 64); err == nil {
			if n >= bestVal {
				bestVal = n
				bestNum = k
			}
			continue
		}
		if k > bestOther {
			bestOther = k
		}
	}
	if bestNum != "" {
		return bestNum
	}
	return bestOther
}

func containsKey(keys []string, want string) bool {
	for _, k := range keys {
		if k == want {
			return true
		}
	}
	return false
}

// LastCLISessionPath is ~/.picoclaw/last_cli_session (or under PICOCLAW_HOME).
func LastCLISessionPath(home string) string {
	if home == "" {
		return lastCLISessionFile
	}
	return filepath.Join(home, lastCLISessionFile)
}

// LoadLastCLISession reads the persisted last session key (trimmed).
func LoadLastCLISession(home string) string {
	b, err := os.ReadFile(LastCLISessionPath(home))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// SaveLastCLISession persists the active session key for the next launch.
func SaveLastCLISession(home, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	path := LastCLISessionPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(key+"\n"), 0o644)
}

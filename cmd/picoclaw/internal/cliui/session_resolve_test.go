package cliui

import "testing"

func TestMostRecentCLISession_PrefersNewestNano(t *testing.T) {
	keys := []string{"cli:100", "cli:default", "cli:999", "telegram:1"}
	got := MostRecentCLISession(keys)
	if got != "cli:999" {
		t.Fatalf("got %q", got)
	}
}

func TestMostRecentCLISession_Empty(t *testing.T) {
	if got := MostRecentCLISession(nil); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestMostRecentCLISession_NumericCompare(t *testing.T) {
	// Lexicographic would prefer cli:99 over cli:100; numeric must prefer 100.
	got := MostRecentCLISession([]string{"cli:99", "cli:100"})
	if got != "cli:100" {
		t.Fatalf("got %q want cli:100", got)
	}
}

func TestResolveCLISession_ExplicitWins(t *testing.T) {
	got := ResolveCLISession(ResolveCLISessionOpts{
		Explicit:    "cli:mine",
		ExplicitSet: true,
		Keys:        []string{"cli:999"},
		Last:        "cli:999",
	})
	if got != "cli:mine" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveCLISession_RestoresLast(t *testing.T) {
	got := ResolveCLISession(ResolveCLISessionOpts{
		ExplicitSet: false,
		Keys:        []string{"cli:1", "cli:2", "cli:9"},
		Last:        "cli:2",
	})
	if got != "cli:2" {
		t.Fatalf("got %q want last", got)
	}
}

func TestResolveCLISession_FallsBackToMostRecent(t *testing.T) {
	got := ResolveCLISession(ResolveCLISessionOpts{
		ExplicitSet: false,
		Keys:        []string{"cli:1", "cli:9"},
		Last:        "cli:missing",
	})
	if got != "cli:9" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveCLISession_NoSessionsUsesFallback(t *testing.T) {
	got := ResolveCLISession(ResolveCLISessionOpts{
		ExplicitSet: false,
		Fallback:    "cli:default",
	})
	if got != "cli:default" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadSaveLastCLISession(t *testing.T) {
	dir := t.TempDir()
	if got := LoadLastCLISession(dir); got != "" {
		t.Fatalf("empty home should be empty, got %q", got)
	}
	if err := SaveLastCLISession(dir, "cli:42"); err != nil {
		t.Fatal(err)
	}
	if got := LoadLastCLISession(dir); got != "cli:42" {
		t.Fatalf("got %q", got)
	}
}

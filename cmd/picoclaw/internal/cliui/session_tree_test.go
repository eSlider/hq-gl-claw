package cliui

import "testing"

func TestPairTurns(t *testing.T) {
	msgs := []ChatMessage{
		{Role: "user", Content: "q1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "q2"},
		{Role: "assistant", Content: "a2"},
	}
	turns := PairTurns(msgs)
	if len(turns) != 2 {
		t.Fatalf("len=%d", len(turns))
	}
	if turns[0].Request != "q1" || turns[0].Response != "a1" {
		t.Fatalf("turn0=%+v", turns[0])
	}
	if turns[1].Request != "q2" || turns[1].Response != "a2" {
		t.Fatalf("turn1=%+v", turns[1])
	}
}

func TestBuildSessionTreeRows_ExpandCurrent(t *testing.T) {
	src := &fakeLister{
		order: []string{"cli:a", "cli:b"},
		keys: map[string][]ChatMessage{
			"cli:a": {
				{Role: "user", Content: "hello there"},
				{Role: "assistant", Content: "hi back"},
			},
			"cli:b": {
				{Role: "user", Content: "other"},
			},
		},
	}
	rows := BuildSessionTreeRows(src, "cli:a", 40, nil)
	var sessions, reqs, resps int
	for _, r := range rows {
		switch r.Kind {
		case TreeRowSession:
			sessions++
		case TreeRowRequest:
			reqs++
			if r.SessionKey != "cli:a" {
				t.Fatalf("request under wrong session: %q", r.SessionKey)
			}
		case TreeRowResponse:
			resps++
		}
	}
	if sessions != 2 {
		t.Fatalf("sessions=%d", sessions)
	}
	if reqs != 1 || resps != 1 {
		t.Fatalf("reqs=%d resps=%d (only current expanded)", reqs, resps)
	}
}

func TestBuildSessionTreeRows_Collapse(t *testing.T) {
	src := &fakeLister{
		order: []string{"cli:a"},
		keys: map[string][]ChatMessage{
			"cli:a": {
				{Role: "user", Content: "q"},
				{Role: "assistant", Content: "a"},
			},
		},
	}
	rows := BuildSessionTreeRows(src, "cli:a", 40, map[string]bool{"cli:a": false})
	for _, r := range rows {
		if r.Kind != TreeRowSession {
			t.Fatalf("expected collapsed, got %+v", r)
		}
	}
}

func TestHitTestPane(t *testing.T) {
	c := ComputeChrome(80, 24, 3)
	if HitTestPane(c, c.Result.X+1, c.Result.Y+1) != FocusResult {
		t.Fatal("result")
	}
	if HitTestPane(c, c.Sessions.X+1, c.Sessions.Y+1) != FocusSessions {
		t.Fatal("sessions")
	}
	if HitTestPane(c, c.Input.X+1, c.Input.Y+1) != FocusInput {
		t.Fatal("input")
	}
	if HitTestPane(c, -1, -1) != FocusNone {
		t.Fatal("miss")
	}
}

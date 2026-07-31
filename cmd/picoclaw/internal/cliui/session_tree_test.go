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
	rows, forest := BuildSessionTreeRows(src, "cli:a", 40, nil, nil, nil)
	if forest["cli:a"] == nil || forest["cli:b"] == nil {
		t.Fatal("forest missing roots")
	}
	var sessions, reqs, resps int
	var maxDepth int
	for _, r := range rows {
		if r.Depth > maxDepth {
			maxDepth = r.Depth
		}
		switch r.Kind {
		case TreeRowSession:
			sessions++
		case TreeRowRequest:
			reqs++
		case TreeRowResponse:
			resps++
		}
	}
	if sessions != 2 {
		t.Fatalf("sessions=%d", sessions)
	}
	// Inactive sessions stay title-only stubs until expanded.
	if len(forest["cli:b"].Children) != 0 {
		t.Fatalf("inactive session should be lazy stub, kids=%d", len(forest["cli:b"].Children))
	}
	// cli:a expanded (nested req+resp); cli:b collapsed at session only when default
	if reqs < 1 || resps < 1 {
		t.Fatalf("reqs=%d resps=%d", reqs, resps)
	}
	if maxDepth < 2 {
		t.Fatalf("expected nested depth>=2, got %d", maxDepth)
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
	rows, forest := BuildSessionTreeRows(src, "cli:a", 40, nil, map[string]bool{"cli:a": false}, nil)
	root := forest["cli:a"]
	if root == nil {
		t.Fatal("no root")
	}
	if len(root.Children) != 0 {
		t.Fatalf("collapsed session should not load turns, kids=%d", len(root.Children))
	}
	// Only session row when collapsed via session key.
	if len(rows) != 1 || rows[0].Kind != TreeRowSession {
		t.Fatalf("expected collapsed session only, got %+v", rows)
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

package cliui

import (
	"strings"
	"testing"
)

func TestBuildConvTreeFromHistory_Nested(t *testing.T) {
	msgs := []ChatMessage{
		{Role: "user", Content: "q1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "q2"},
		{Role: "assistant", Content: "a2"},
	}
	root := BuildConvTreeFromHistory("cli:t", "", msgs, nil)
	if root.Kind != TreeRowSession {
		t.Fatalf("root kind=%v", root.Kind)
	}
	if len(root.Children) != 1 || root.Children[0].Kind != TreeRowRequest {
		t.Fatalf("expected one request under session, got %+v", root.Children)
	}
	req1 := root.Children[0]
	if req1.Content != "q1" || len(req1.Children) != 1 {
		t.Fatalf("req1=%+v", req1)
	}
	resp1 := req1.Children[0]
	if resp1.Kind != TreeRowResponse || resp1.Content != "a1" {
		t.Fatalf("resp1=%+v", resp1)
	}
	if len(resp1.Children) != 1 || resp1.Children[0].Content != "q2" {
		t.Fatalf("q2 should nest under a1: %+v", resp1.Children)
	}
	req2 := resp1.Children[0]
	if len(req2.Children) != 1 || req2.Children[0].Content != "a2" {
		t.Fatalf("a2 under q2: %+v", req2.Children)
	}
}

func TestSubmitParent_Rules(t *testing.T) {
	root := BuildConvTreeFromHistory("cli:t", "t", []ChatMessage{
		{Role: "user", Content: "q"},
		{Role: "assistant", Content: "a"},
	}, nil)
	req := root.Children[0]
	resp := req.Children[0]

	if p := SubmitParent(root); p != root {
		t.Fatal("session → session")
	}
	if p := SubmitParent(resp); p != resp {
		t.Fatal("response → response")
	}
	if p := SubmitParent(req); p != root {
		t.Fatalf("request → parent session, got %v", p)
	}
}

func TestAttachRequest_BranchUnderResponse(t *testing.T) {
	gen := &idGen{}
	root := BuildConvTreeFromHistory("cli:t", "t", []ChatMessage{
		{Role: "user", Content: "q"},
		{Role: "assistant", Content: "a"},
	}, gen)
	resp := root.Children[0].Children[0]
	r2 := AttachRequest(resp, "branch", gen)
	r3 := AttachRequest(resp, "branch2", gen)
	if len(resp.Children) != 2 {
		t.Fatalf("want 2 requests under response, got %d", len(resp.Children))
	}
	if r2.Parent != resp || r3.Parent != resp {
		t.Fatal("parent link")
	}
}

func TestPathMessages(t *testing.T) {
	root := BuildConvTreeFromHistory("cli:t", "t", []ChatMessage{
		{Role: "user", Content: "q1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "q2"},
	}, nil)
	req2 := root.Children[0].Children[0].Children[0]
	msgs := PathMessages(req2)
	if len(msgs) != 3 {
		t.Fatalf("msgs=%+v", msgs)
	}
	if msgs[0].Role != "user" || msgs[0].Content != "q1" {
		t.Fatalf("0=%+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[2].Content != "q2" {
		t.Fatalf("path=%+v", msgs)
	}
	before := PathMessagesBefore(req2.Parent) // response a1
	if len(before) != 2 {
		t.Fatalf("before=%+v", before)
	}
}

func TestFlattenConvTree_FlatList(t *testing.T) {
	root := BuildConvTreeFromHistory("cli:t", "hello", []ChatMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
		{Role: "user", Content: "next"},
		{Role: "assistant", Content: "ok"},
	}, nil)
	rows := FlattenConvTree(root, "cli:t", 40, nil)
	if len(rows) < 5 {
		t.Fatalf("rows=%d", len(rows))
	}
	joined := ""
	for _, r := range rows {
		joined += r.Label + "\n"
		if strings.ContainsAny(r.Label, "├└│") {
			t.Fatalf("tree connectors in flat list: %q", r.Label)
		}
	}
	// Depth still tracked for logic; labels stay left-aligned (no indent).
	if rows[1].Depth != 1 || rows[2].Depth != 2 {
		t.Fatalf("depths: %d %d", rows[1].Depth, rows[2].Depth)
	}
	if strings.HasPrefix(rows[3].Label, " ") {
		t.Fatalf("expected no indent: %q", rows[3].Label)
	}
	if !strings.Contains(joined, "●") || !strings.Contains(joined, "↑") || !strings.Contains(joined, "↓") {
		t.Fatalf("missing glyphs: %s", joined)
	}
}

func TestFlattenConvTree_NoDuplicateKindGlyphs(t *testing.T) {
	root := BuildConvTreeFromHistory("cli:t", "t", []ChatMessage{
		{Role: "user", Content: "q"},
		{Role: "assistant", Content: "a"},
	}, nil)
	rows := FlattenConvTree(root, "cli:t", 40, nil)
	if !strings.Contains(rows[0].Label, "●") {
		t.Fatalf("session=%q", rows[0].Label)
	}
	if strings.Count(rows[1].Label, "↑") != 1 {
		t.Fatalf("request icons=%q", rows[1].Label)
	}
	if strings.Count(rows[2].Label, "↓") != 1 {
		t.Fatalf("response icons=%q", rows[2].Label)
	}
}

func TestStyleSessionRow_VisibleContrast(t *testing.T) {
	plain := StyleSessionRow("● hello", TreeRowSession, false, 12)
	if !strings.Contains(plain, "bg:navy") || !strings.Contains(plain, "fg:white") {
		t.Fatalf("session style: %q", plain)
	}
	req := StyleSessionRow("↑ ask", TreeRowRequest, false, 0)
	if !strings.Contains(req, "bg:lightgreen") || !strings.Contains(req, "fg:black") {
		t.Fatalf("request style: %q", req)
	}
	sel := StyleSessionRow("↓ ans", TreeRowResponse, true, 0)
	if !strings.Contains(sel, "bg:yellow") || !strings.Contains(sel, "fg:black") {
		t.Fatalf("selected style: %q", sel)
	}
}

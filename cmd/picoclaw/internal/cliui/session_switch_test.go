package cliui

import (
	"strings"
	"testing"
)

func TestSyncSessions_SwitchesSelectionAndCollapsesOther(t *testing.T) {
	src := &fakeLister{
		order: []string{"cli:a", "cli:b"},
		keys: map[string][]ChatMessage{
			"cli:a": {
				{Role: "user", Content: "ask-a"},
				{Role: "assistant", Content: "ans-a"},
			},
			"cli:b": {
				{Role: "user", Content: "ask-b"},
				{Role: "assistant", Content: "ans-b"},
			},
		},
	}
	ui := NewAgentTUI("You: ")
	ui.width, ui.height = 80, 24
	ui.SyncSessions(src, "cli:a")

	rootA := ui.forest["cli:a"]
	if rootA == nil {
		t.Fatal("no forest a")
	}
	tipA := DeepestTip(rootA)
	ui.mu.Lock()
	ui.selectedID = tipA.ID
	ui.rebuildTreeLocked()
	ui.mu.Unlock()

	ui.SyncSessions(src, "cli:b")

	ui.mu.Lock()
	defer ui.mu.Unlock()
	if ui.currentKey != "cli:b" {
		t.Fatalf("current=%q", ui.currentKey)
	}
	sel := ui.selectedNodeLocked()
	if sel == nil || sel.Session != "cli:b" {
		t.Fatalf("selection still on other session: %+v", sel)
	}
	var aRows, bDepth int
	for _, r := range ui.treeRows {
		if r.SessionKey == "cli:a" {
			aRows++
		}
		if r.SessionKey == "cli:b" && r.Depth > bDepth {
			bDepth = r.Depth
		}
	}
	if aRows != 1 {
		t.Fatalf("session A should be collapsed to 1 row, got %d", aRows)
	}
	if bDepth < 2 {
		t.Fatalf("session B should show nested tip, maxDepth=%d", bDepth)
	}
	if !strings.Contains(ui.transcriptPlain, "ask-b") && !strings.Contains(ui.plainBuf, "ask-b") {
		t.Fatalf("expected B transcript, plain=%q transcript=%q", ui.plainBuf, ui.transcriptPlain)
	}
}

func TestRebuildTree_IgnoresForeignSelectedID(t *testing.T) {
	src := &fakeLister{
		order: []string{"cli:a", "cli:b"},
		keys: map[string][]ChatMessage{
			"cli:a": {{Role: "user", Content: "a"}},
			"cli:b": {{Role: "user", Content: "b"}, {Role: "assistant", Content: "bb"}},
		},
	}
	ui := NewAgentTUI("")
	ui.SyncSessions(src, "cli:a")
	idA := DeepestTip(ui.forest["cli:a"]).ID

	ui.mu.Lock()
	ui.currentKey = "cli:b"
	ui.selectedID = idA // stale foreign id
	ui.treeExpanded["cli:b"] = true
	ui.treeExpanded["cli:a"] = true
	ui.rebuildTreeLocked()
	sel := ui.selectedNodeLocked()
	ui.mu.Unlock()

	if sel == nil || sel.Session != "cli:b" {
		t.Fatalf("wanted selection in cli:b, got %+v", sel)
	}
}

func TestActivateRequest_SwitchesSession(t *testing.T) {
	src := &fakeLister{
		order: []string{"cli:a", "cli:b"},
		keys: map[string][]ChatMessage{
			"cli:a": {{Role: "user", Content: "from-a"}},
			"cli:b": {{Role: "user", Content: "from-b"}},
		},
	}
	ui := NewAgentTUI("")
	ui.SyncSessions(src, "cli:a")

	ui.mu.Lock()
	ui.treeExpanded["cli:b"] = true
	if root := ui.forest["cli:b"]; root != nil {
		ui.treeExpanded[root.ID] = true
		for _, c := range root.Children {
			ui.treeExpanded[c.ID] = true
		}
	}
	ui.rebuildTreeLocked()
	var reqRow SessionTreeRow
	for _, r := range ui.treeRows {
		if r.SessionKey == "cli:b" && r.Kind == TreeRowRequest {
			reqRow = r
			break
		}
	}
	ui.mu.Unlock()
	if reqRow.NodeID == "" {
		t.Fatal("no request row for B")
	}

	ui.activateTreeRow(reqRow)

	ui.mu.Lock()
	defer ui.mu.Unlock()
	if ui.currentKey != "cli:b" {
		t.Fatalf("currentKey=%q after activate request in B", ui.currentKey)
	}
	if ui.selectedID != reqRow.NodeID {
		t.Fatalf("selected=%q want %q", ui.selectedID, reqRow.NodeID)
	}
	if ui.input.Text != "from-b" {
		t.Fatalf("autofill=%q", ui.input.Text)
	}
}

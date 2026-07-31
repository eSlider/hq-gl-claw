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

func TestActivateSession_KeepsSiblingsListed(t *testing.T) {
	src := &fakeLister{
		order: []string{"cli:100", "cli:200", "cli:300"},
		keys: map[string][]ChatMessage{
			"cli:100": {{Role: "user", Content: "a"}, {Role: "assistant", Content: "aa"}},
			"cli:200": {{Role: "user", Content: "b"}, {Role: "assistant", Content: "bb"}},
			"cli:300": {{Role: "user", Content: "c"}, {Role: "assistant", Content: "cc"}},
		},
	}
	ui := NewAgentTUI("")
	ui.width, ui.height = 80, 24
	ui.SyncSessions(src, "cli:300")

	// Activate oldest session (would previously jump to list top).
	ui.mu.Lock()
	var oldRow SessionTreeRow
	for _, r := range ui.treeRows {
		if r.Kind == TreeRowSession && r.SessionKey == "cli:100" {
			oldRow = r
			break
		}
	}
	ui.mu.Unlock()
	if oldRow.NodeID == "" {
		t.Fatal("missing cli:100 row")
	}
	ui.activateTreeRow(oldRow)

	ui.mu.Lock()
	defer ui.mu.Unlock()
	if ui.currentKey != "cli:100" {
		t.Fatalf("current=%q", ui.currentKey)
	}
	var sessionKeys []string
	for _, r := range ui.treeRows {
		if r.Kind == TreeRowSession {
			sessionKeys = append(sessionKeys, r.SessionKey)
		}
	}
	if len(sessionKeys) != 3 {
		t.Fatalf("expected 3 session roots, got %v", sessionKeys)
	}
	// Stable newest-first order preserved.
	want := []string{"cli:300", "cli:200", "cli:100"}
	for i, k := range want {
		if sessionKeys[i] != k {
			t.Fatalf("order=%v want %v", sessionKeys, want)
		}
	}
	// Selected session must not be forced to index 0 of treeRows as a reorder.
	if ui.treeRows[0].SessionKey == "cli:100" && ui.treeRows[0].Kind == TreeRowSession {
		// Only OK if it is genuinely newest — it is not.
		t.Fatal("active session jumped to top of list")
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

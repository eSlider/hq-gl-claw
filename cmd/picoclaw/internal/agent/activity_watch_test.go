package agent

import (
	"testing"

	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/cliui"
	pkgagent "github.com/sipeed/picoclaw/pkg/agent"
	runtimeevents "github.com/sipeed/picoclaw/pkg/events"
)

func TestApplyAgentActivityEvent_ToolLifecycle(t *testing.T) {
	ui := cliui.NewAgentTUI("")
	applyAgentActivityEvent(ui, runtimeevents.Event{
		Kind:    runtimeevents.KindAgentToolExecStart,
		Payload: pkgagent.ToolExecStartPayload{Tool: "read_file"},
	})
	phase, detail := ui.ActivitySnapshot()
	if phase != cliui.ActivityTool || detail != "read_file" {
		t.Fatalf("start phase=%q detail=%q", phase, detail)
	}

	applyAgentActivityEvent(ui, runtimeevents.Event{
		Kind:    runtimeevents.KindAgentToolExecEnd,
		Payload: pkgagent.ToolExecEndPayload{Tool: "read_file"},
	})
	phase, _ = ui.ActivitySnapshot()
	if phase != cliui.ActivityLLM {
		t.Fatalf("after tool end want llm, got %q", phase)
	}

	applyAgentActivityEvent(ui, runtimeevents.Event{Kind: runtimeevents.KindAgentTurnEnd})
	phase, _ = ui.ActivitySnapshot()
	if phase != cliui.ActivityIdle {
		t.Fatalf("turn end want idle, got %q", phase)
	}
}

func TestApplyAgentActivityEvent_LLM(t *testing.T) {
	ui := cliui.NewAgentTUI("")
	applyAgentActivityEvent(ui, runtimeevents.Event{
		Kind:    runtimeevents.KindAgentLLMRequest,
		Payload: pkgagent.LLMRequestPayload{Model: "bonsai"},
	})
	phase, detail := ui.ActivitySnapshot()
	if phase != cliui.ActivityLLM || detail != "bonsai" {
		t.Fatalf("llm phase=%q detail=%q", phase, detail)
	}
}

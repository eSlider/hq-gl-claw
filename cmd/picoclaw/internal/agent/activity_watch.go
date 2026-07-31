package agent

import (
	"context"
	"strings"

	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/cliui"
	pkgagent "github.com/sipeed/picoclaw/pkg/agent"
	runtimeevents "github.com/sipeed/picoclaw/pkg/events"
)

// watchAgentActivity mirrors long-running agent phases into the TUI status pane.
func watchAgentActivity(ctx context.Context, al *pkgagent.AgentLoop, ui *cliui.AgentTUI) {
	if al == nil || ui == nil {
		return
	}
	bus := al.RuntimeEventBus()
	if bus == nil {
		return
	}
	sub, ch, err := bus.Channel().OfKind(
		runtimeevents.KindAgentLLMRequest,
		runtimeevents.KindAgentToolExecStart,
		runtimeevents.KindAgentToolExecEnd,
		runtimeevents.KindAgentContextCompress,
		runtimeevents.KindAgentSubTurnSpawn,
		runtimeevents.KindAgentTurnEnd,
		runtimeevents.KindAgentError,
	).SubscribeChan(ctx, runtimeevents.SubscribeOptions{
		Name:         "cli-status-activity",
		Buffer:       64,
		Backpressure: runtimeevents.DropOldest,
	})
	if err != nil {
		return
	}
	go func() {
		defer func() { _ = sub.Close() }()
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-ch:
				if !ok {
					return
				}
				applyAgentActivityEvent(ui, evt)
			}
		}
	}()
}

func applyAgentActivityEvent(ui *cliui.AgentTUI, evt runtimeevents.Event) {
	if ui == nil {
		return
	}
	switch evt.Kind {
	case runtimeevents.KindAgentLLMRequest:
		model := ""
		if p, ok := evt.Payload.(pkgagent.LLMRequestPayload); ok {
			model = strings.TrimSpace(p.Model)
		}
		ui.SetActivity(cliui.ActivityLLM, model)
	case runtimeevents.KindAgentToolExecStart:
		name := "tool"
		if p, ok := evt.Payload.(pkgagent.ToolExecStartPayload); ok && strings.TrimSpace(p.Tool) != "" {
			name = strings.TrimSpace(p.Tool)
		}
		ui.BeginToolActivity(name)
	case runtimeevents.KindAgentToolExecEnd:
		name := ""
		if p, ok := evt.Payload.(pkgagent.ToolExecEndPayload); ok {
			name = strings.TrimSpace(p.Tool)
		}
		ui.EndToolActivity(name)
	case runtimeevents.KindAgentContextCompress:
		ui.SetActivity(cliui.ActivityCompress, "")
	case runtimeevents.KindAgentSubTurnSpawn:
		label := ""
		if p, ok := evt.Payload.(pkgagent.SubTurnSpawnPayload); ok {
			label = strings.TrimSpace(p.Label)
			if label == "" {
				label = strings.TrimSpace(p.AgentID)
			}
		}
		ui.SetActivity(cliui.ActivitySubagent, label)
	case runtimeevents.KindAgentTurnEnd, runtimeevents.KindAgentError:
		ui.ClearActivity()
	}
}

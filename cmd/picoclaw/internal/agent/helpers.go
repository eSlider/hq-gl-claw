package agent

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ergochat/readline"

	"github.com/sipeed/picoclaw/cmd/picoclaw/internal"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/cliui"
	"github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/session"
)

func agentCmd(message, sessionKey string, sessionSet bool, model string, debug bool) error {
	cfg, err := internal.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	logger.ConfigureFromEnv()

	if debug {
		logger.SetLevel(logger.DEBUG)
		fmt.Println("🔍 Debug mode enabled")
	}

	if model != "" {
		cfg.Agents.Defaults.ModelName = model
	}
	// Ensure local OpenAI-compatible bonsai endpoint is available to cycle to.
	cliui.EnsureBonsaiLocal(cfg, false)

	if err = cliui.EnableCLIAgentStreaming(cfg); err != nil {
		return fmt.Errorf("enable cli streaming: %w", err)
	}

	provider, modelID, err := providers.CreateProvider(cfg)
	if err != nil {
		return fmt.Errorf("error creating provider: %w", err)
	}

	// Use the resolved model ID from provider creation
	if modelID != "" {
		cfg.Agents.Defaults.ModelName = modelID
	}

	msgBus := bus.NewMessageBus()
	defer msgBus.Close()
	agentLoop := agent.NewAgentLoop(cfg, msgBus, provider)
	defer agentLoop.Close()

	// Print agent startup info (only for interactive mode)
	startupInfo := agentLoop.GetStartupInfo()
	toolsInfo, ok := startupInfo["tools"].(map[string]any)
	if !ok {
		toolsInfo = nil
	}
	skillsInfo, ok := startupInfo["skills"].(map[string]any)
	if !ok {
		skillsInfo = nil
	}
	logFields := map[string]any{}
	if toolsInfo != nil {
		logFields["tools_count"] = toolsInfo["count"]
	}
	if skillsInfo != nil {
		logFields["skills_total"] = skillsInfo["total"]
		logFields["skills_available"] = skillsInfo["available"]
	}
	logger.InfoCF("agent", "Agent initialized", logFields)

	home := internal.GetPicoclawHome()
	lister := newAgentSessionLister(agentLoop)
	sessionKey = cliui.ResolveCLISession(cliui.ResolveCLISessionOpts{
		Explicit:    sessionKey,
		ExplicitSet: sessionSet,
		Keys:        lister.ListSessions(),
		Last:        cliui.LoadLastCLISession(home),
		Fallback:    cliui.DefaultCLISession,
	})
	_ = cliui.SaveLastCLISession(home, sessionKey)

	if message != "" {
		return runOneTurn(agentLoop, msgBus, message, sessionKey)
	}

	fmt.Printf("%s Interactive mode (Ctrl+C exit · Ctrl+N new session)\n\n", internal.Logo)
	interactiveMode(agentLoop, msgBus, sessionKey)

	return nil
}

func runOneTurn(agentLoop *agent.AgentLoop, msgBus *bus.MessageBus, message, sessionKey string) error {
	display := cliui.NewLiveDisplay(os.Stdout, os.Stderr, internal.Logo)
	msgBus.SetStreamDelegate(cliui.NewStreamDelegate(display))
	display.StartPrompt(message)

	ctx := context.Background()
	response, err := agentLoop.ProcessDirect(ctx, message, sessionKey)
	if err != nil {
		display.Cancel(ctx)
		return fmt.Errorf("error processing message: %w", err)
	}
	display.Finish(response)
	return nil
}

func interactiveMode(agentLoop *agent.AgentLoop, msgBus *bus.MessageBus, sessionKey string) {
	if cliui.PanesEnabled() {
		if err := paneInteractiveMode(agentLoop, msgBus, sessionKey); err != nil {
			fmt.Printf("Pane UI error: %v\nFalling back to readline...\n", err)
		} else {
			fmt.Println("Goodbye!")
			return
		}
	}

	prompt := fmt.Sprintf("%s You: ", internal.Logo)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          prompt,
		HistoryFile:     filepath.Join(os.TempDir(), ".picoclaw_history"),
		HistoryLimit:    100,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		fmt.Printf("Error initializing readline: %v\n", err)
		fmt.Println("Falling back to simple input mode...")
		simpleInteractiveMode(agentLoop, msgBus, sessionKey)
		return
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt || err == io.EOF {
				fmt.Println("\nGoodbye!")
				return
			}
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			return
		}

		if err := runOneTurn(agentLoop, msgBus, input, sessionKey); err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		fmt.Fprintln(os.Stdout)
	}
}

func paneInteractiveMode(agentLoop *agent.AgentLoop, msgBus *bus.MessageBus, sessionKey string) error {
	ui, err := cliui.NewPaneUI(fmt.Sprintf("%s You: ", internal.Logo))
	if err != nil {
		return err
	}

	home := internal.GetPicoclawHome()
	lister := newAgentSessionLister(agentLoop)
	ui.SyncSessions(lister, sessionKey)
	ui.ShowSessionHistory(lister.GetHistory(sessionKey))
	ui.SetEndpoint(cliui.EndpointFromConfig(agentLoop.GetConfig()))
	_ = cliui.SaveLastCLISession(home, sessionKey)

	watchCtx, watchCancel := context.WithCancel(context.Background())
	defer watchCancel()
	watchAgentActivity(watchCtx, agentLoop, ui)

	return ui.Run(func(ev cliui.PaneEvent) error {
		switch ev.Action {
		case cliui.KeyActionSwitchSession:
			sessionKey = ev.Payload
			_ = cliui.SaveLastCLISession(home, sessionKey)
			ui.SyncSessions(lister, sessionKey)
			return nil
		case cliui.KeyActionNewSession:
			prev := sessionKey
			sessionKey = ev.Payload
			if sessionKey == "" {
				sessionKey = cliui.NewSessionKey()
			}
			if prev != "" {
				ui.RetainSession(prev)
			}
			_ = cliui.SaveLastCLISession(home, sessionKey)
			ui.SyncSessions(lister, sessionKey)
			return nil
		case cliui.KeyActionCycleModel:
			return cycleAgentModel(agentLoop, ui)
		case cliui.KeyActionSubmit:
			if ev.SessionKey != "" {
				sessionKey = ev.SessionKey
			}
			streamer := cliui.NewPaneStreamer(ui)
			streamer.Start(ev.Payload)
			ui.SetActivity(cliui.ActivityLLM, "")
			msgBus.SetStreamDelegate(cliui.NewPaneStreamDelegate(streamer))
			ctx := context.Background()
			// Install branch path so ProcessDirect continues from the selected parent.
			lister.SetHistory(sessionKey, ev.HistoryBefore)
			response, err := agentLoop.ProcessDirect(ctx, ev.Payload, sessionKey)
			if err != nil {
				streamer.Cancel(ctx)
				ui.ClearActivity()
				return err
			}
			streamer.Finish(response)
			ui.ClearActivity()
			ui.CompleteRequest(ev.NodeID, response)
			_ = cliui.SaveLastCLISession(home, sessionKey)
			ui.SyncSessions(lister, sessionKey)
			return nil
		default:
			return nil
		}
	})
}

func cycleAgentModel(agentLoop *agent.AgentLoop, ui *cliui.AgentTUI) error {
	cfg := agentLoop.GetConfig()
	if cfg == nil {
		return fmt.Errorf("no config")
	}
	current := cfg.Agents.Defaults.GetModelName()
	next := cliui.NextModelName(cfg, current)
	if next == "" || next == current {
		return fmt.Errorf("no other enabled models to cycle")
	}
	cfg.Agents.Defaults.ModelName = next
	provider, _, err := providers.CreateProvider(cfg)
	if err != nil {
		cfg.Agents.Defaults.ModelName = current
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := agentLoop.ReloadProviderAndConfig(ctx, provider, cfg); err != nil {
		cfg.Agents.Defaults.ModelName = current
		return err
	}
	ep := cliui.EndpointFromConfig(cfg)
	ui.SetEndpoint(ep)
	ui.SetProgressText(fmt.Sprintf("model → %s", cliui.FormatEndpoint(ep)))
	return nil
}

// agentSessionLister adapts AgentLoop session store to cliui.SessionLister.
type agentSessionLister struct {
	store session.SessionStore
}

func newAgentSessionLister(agentLoop *agent.AgentLoop) *agentSessionLister {
	var store session.SessionStore
	if agentLoop != nil && agentLoop.GetRegistry() != nil {
		if a := agentLoop.GetRegistry().GetDefaultAgent(); a != nil {
			store = a.Sessions
		}
	}
	return &agentSessionLister{store: store}
}

func (l *agentSessionLister) ListSessions() []string {
	if l == nil || l.store == nil {
		return nil
	}
	return l.store.ListSessions()
}

func (l *agentSessionLister) GetHistory(key string) []cliui.ChatMessage {
	if l == nil || l.store == nil {
		return nil
	}
	raw := l.store.GetHistory(key)
	out := make([]cliui.ChatMessage, 0, len(raw))
	for _, m := range raw {
		out = append(out, cliui.ChatMessage{Role: m.Role, Content: m.Content})
	}
	return out
}

func (l *agentSessionLister) SetHistory(key string, msgs []cliui.ChatMessage) {
	if l == nil || l.store == nil {
		return
	}
	hist := make([]providers.Message, 0, len(msgs))
	for _, m := range msgs {
		hist = append(hist, providers.Message{Role: m.Role, Content: m.Content})
	}
	l.store.SetHistory(key, hist)
}

func simpleInteractiveMode(agentLoop *agent.AgentLoop, msgBus *bus.MessageBus, sessionKey string) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(fmt.Sprintf("%s You: ", internal.Logo))
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println("\nGoodbye!")
				return
			}
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			return
		}

		if err := runOneTurn(agentLoop, msgBus, input, sessionKey); err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		fmt.Fprintln(os.Stdout)
	}
}

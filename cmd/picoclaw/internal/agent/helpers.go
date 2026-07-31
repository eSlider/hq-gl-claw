package agent

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
	_ = cliui.SaveLastCLISession(home, sessionKey)

	return ui.Run(func(ev cliui.PaneEvent) error {
		switch ev.Action {
		case cliui.KeyActionSwitchSession:
			sessionKey = ev.Payload
			_ = cliui.SaveLastCLISession(home, sessionKey)
			ui.SyncSessions(lister, sessionKey)
			ui.ShowSessionHistory(lister.GetHistory(sessionKey))
			return nil
		case cliui.KeyActionNewSession:
			prev := sessionKey
			sessionKey = ev.Payload
			if sessionKey == "" {
				sessionKey = cliui.NewSessionKey()
			}
			_ = cliui.SaveLastCLISession(home, sessionKey)
			// Retain previous + new so Ctrl+N never drops the prior row from the tree.
			ui.SyncSessions(lister, sessionKey)
			if prev != "" {
				ui.RetainSession(prev)
			}
			ui.ShowSessionHistory(nil)
			return nil
		case cliui.KeyActionSubmit:
			streamer := cliui.NewPaneStreamer(ui)
			streamer.Start(ev.Payload)
			msgBus.SetStreamDelegate(cliui.NewPaneStreamDelegate(streamer))
			ctx := context.Background()
			response, err := agentLoop.ProcessDirect(ctx, ev.Payload, sessionKey)
			if err != nil {
				streamer.Cancel(ctx)
				return err
			}
			streamer.Finish(response)
			_ = cliui.SaveLastCLISession(home, sessionKey)
			ui.SyncSessions(lister, sessionKey)
			ui.ShowSessionHistory(lister.GetHistory(sessionKey))
			return nil
		default:
			return nil
		}
	})
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

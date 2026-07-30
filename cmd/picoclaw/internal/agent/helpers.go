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
)

func agentCmd(message, sessionKey, model string, debug bool) error {
	if sessionKey == "" {
		sessionKey = "cli:default"
	}

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

	if message != "" {
		return runOneTurn(agentLoop, msgBus, message, sessionKey)
	}

	fmt.Printf("%s Interactive mode (Ctrl+C to exit)\n\n", internal.Logo)
	interactiveMode(agentLoop, msgBus, sessionKey)

	return nil
}

func runOneTurn(agentLoop *agent.AgentLoop, msgBus *bus.MessageBus, message, sessionKey string) error {
	display := cliui.NewLiveDisplay(os.Stdout, os.Stderr, internal.Logo)
	msgBus.SetStreamDelegate(cliui.NewStreamDelegate(display))
	display.Start()

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
	return ui.Run(func(msg string) error {
		streamer := cliui.NewPaneStreamer(ui)
		streamer.Start()
		msgBus.SetStreamDelegate(cliui.NewPaneStreamDelegate(streamer))
		ctx := context.Background()
		response, err := agentLoop.ProcessDirect(ctx, msg, sessionKey)
		if err != nil {
			streamer.Cancel(ctx)
			return err
		}
		streamer.Finish(response)
		return nil
	})
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

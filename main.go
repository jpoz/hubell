package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jpoz/hubell/internal/auth"
	"github.com/jpoz/hubell/internal/github"
	"github.com/jpoz/hubell/internal/notify"
	"github.com/jpoz/hubell/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Create context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Initialize token store
	tokenStore := auth.NewTokenStore()

	// Load token from disk or environment variable
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = tokenStore.Load()
	}

	// If no token found, prompt user
	if token == "" {
		var err error
		token, err = auth.PromptForToken()
		if err != nil {
			return fmt.Errorf("failed to get token: %w", err)
		}

		// Save token for future use
		if err := tokenStore.Save(token); err != nil {
			return fmt.Errorf("failed to save token: %w", err)
		}
		fmt.Println("✓ Token saved to ~/.config/hubell/token")
		fmt.Println()
	}

	// Create GitHub client
	client := github.NewClient(token)

	// Get authenticated user for PR status polling
	user, err := client.GetAuthenticatedUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to get authenticated user: %w", err)
	}

	// Create progress channel for loading checklist
	progressCh := make(chan github.LoadingProgress, 8)

	// Create poller with 30-second interval
	poller := github.NewPoller(client, 30*time.Second, user.Login, progressCh)
	pollCh := poller.Start(ctx)

	// Send test notification on startup
	notify.SendDesktopNotification("hubell", "Application started successfully!")

	// Create and run TUI
	model := tui.New(ctx, client, pollCh, progressCh)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

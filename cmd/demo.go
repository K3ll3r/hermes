package cmd

import (
	"github.com/TsekNet/hermes/internal/config"
	"github.com/spf13/cobra"
)

func demoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "demo",
		Short: "Show a demo notification",
		Run: func(_ *cobra.Command, _ []string) {
			cfg := demoConfig()
			waitForDND(cfg)
			runUI(cfg)
		},
	}
}

func demoConfig() *config.NotificationConfig {
	cfg := &config.NotificationConfig{
		Heading:        "Welcome to Hermes",
		Message:        "Your cross-platform notification system is ready. Click a button below or press ESC to close.",
		Title:          "NVIDIA IT",
		AccentColor:    "#76B900",
		TimeoutSeconds: 60,
		TimeoutValue:   "timeout",
		EscValue:       "dismiss",
		HelpURL:        "https://gitlab.com/dtsekhanskiy/hermes",
		Buttons: []config.Button{
			{
				Label: "Explore",
				Style: "secondary",
				Dropdown: []config.DropdownOption{
					{Label: "View Docs", Value: "url:https://gitlab.com/dtsekhanskiy/hermes#readme"},
					{Label: "View Source", Value: "url:https://gitlab.com/dtsekhanskiy/hermes"},
				},
			},
			{Label: "Got it", Value: "ok", Style: "primary"},
		},
	}
	cfg.ApplyDefaults()
	return cfg
}

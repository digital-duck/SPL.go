package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/digital-duck/spl20go/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize ~/.spl/ directory and write default config.yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		cfgPath := cfg.Path()
		if cfgPath == "" {
			return fmt.Errorf("init: cannot determine home directory")
		}

		// Check if config already exists
		if _, err := os.Stat(cfgPath); err == nil {
			fmt.Printf("Config already exists at %s — use 'gspl config show' to view\n", cfgPath)
			return nil
		}

		// Create directory and write default config
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("init: %w", err)
		}

		fmt.Printf("Initialized %s\n", cfgPath)
		return nil
	},
}

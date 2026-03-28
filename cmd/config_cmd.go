package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/digital-duck/spl20go/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage SPL runtime configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print current configuration as YAML",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("config show: %w", err)
		}
		fmt.Print(string(data))
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print path to config file",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		fmt.Println(cfg.Path())
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Print value of a config key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		val := cfg.Get(args[0])
		if val == "" {
			return fmt.Errorf("config: unknown or unset key %q", args[0])
		}
		fmt.Println(val)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config key and save",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		if err := cfg.Set(args[0], args[1]); err != nil {
			return err
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("config: %s = %s\n", args[0], args[1])
		return nil
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
}

package cmd

import (
	"fmt"

	"github.com/digital-duck/spl20go/internal/storage"
	"github.com/spf13/cobra"
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage the SPL in-process memory store (~/.spl/memory.db)",
}

var memoryListCmd = &cobra.Command{
	Use:   "list",
	Short: "Print all keys and values",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := storage.DefaultMemoryStore()
		if err != nil {
			return err
		}
		defer store.Close()

		keys, err := store.ListKeys()
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			fmt.Println("(memory store is empty)")
			return nil
		}
		for _, k := range keys {
			v, _ := store.Get(k)
			fmt.Printf("%s = %s\n", k, v)
		}
		return nil
	},
}

var memoryGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Print value for key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := storage.DefaultMemoryStore()
		if err != nil {
			return err
		}
		defer store.Close()

		v, ok := store.Get(args[0])
		if !ok {
			return fmt.Errorf("memory: key %q not found", args[0])
		}
		fmt.Println(v)
		return nil
	},
}

var memorySetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set key=value and save",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := storage.DefaultMemoryStore()
		if err != nil {
			return err
		}
		defer store.Close()

		if err := store.Set(args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("memory: %s = %s\n", args[0], args[1])
		return nil
	},
}

var memoryDeleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Remove key and save",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := storage.DefaultMemoryStore()
		if err != nil {
			return err
		}
		defer store.Close()

		if _, ok := store.Get(args[0]); !ok {
			return fmt.Errorf("memory: key %q not found", args[0])
		}
		if err := store.Delete(args[0]); err != nil {
			return err
		}
		fmt.Printf("memory: deleted %s\n", args[0])
		return nil
	},
}

func init() {
	memoryCmd.AddCommand(memoryListCmd)
	memoryCmd.AddCommand(memoryGetCmd)
	memoryCmd.AddCommand(memorySetCmd)
	memoryCmd.AddCommand(memoryDeleteCmd)
}

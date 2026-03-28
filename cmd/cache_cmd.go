package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/digital-duck/spl20go/internal/storage"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the SPL prompt cache (SQLite prompt_cache table)",
}

var cacheListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all entries in the prompt cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		ms, err := storage.DefaultMemoryStore()
		if err != nil {
			return fmt.Errorf("cache list: open store: %w", err)
		}
		defer ms.Close()

		entries, err := ms.CacheList()
		if err != nil {
			return fmt.Errorf("cache list: %w", err)
		}

		if len(entries) == 0 {
			fmt.Println("(no cached entries)")
			return nil
		}

		fmt.Printf("%-44s  %-20s  %8s  %-20s  %-20s\n",
			"hash", "model", "tokens", "created_at", "expires_at")
		fmt.Printf("%-44s  %-20s  %8s  %-20s  %-20s\n",
			"----", "-----", "------", "----------", "----------")
		for _, e := range entries {
			fmt.Printf("%-44s  %-20s  %8s  %-20s  %-20s\n",
				e["hash"], e["model"], e["tokens"], e["created_at"], e["expires_at"])
		}
		return nil
	},
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete all rows from the prompt cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		ms, err := storage.DefaultMemoryStore()
		if err != nil {
			return fmt.Errorf("cache clear: open store: %w", err)
		}
		defer ms.Close()

		n, err := ms.CacheClear()
		if err != nil {
			return fmt.Errorf("cache clear: %w", err)
		}

		fmt.Printf("Deleted %d cached entry/entries\n", n)
		return nil
	},
}

func init() {
	cacheCmd.AddCommand(cacheListCmd)
	cacheCmd.AddCommand(cacheClearCmd)
}

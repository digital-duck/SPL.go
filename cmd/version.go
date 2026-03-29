package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the SPL runtime version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("spl-go — SPL 2.0 Go runtime v0.1.0")
	},
}

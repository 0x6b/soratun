package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/soracom/soratun"
	"github.com/spf13/cobra"
)

func upCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "up",
		Aliases: []string{"u"},
		Short:   "Setup SORACOM Arc interface",
		Args:    cobra.NoArgs,
		PreRun:  initSoratun,
		Run: func(cmd *cobra.Command, args []string) {
			if Config.ArcSession == nil {
				log.Fatal("Failed to determine connection information. Please bootstrap or create a new session from the user console.")
			}

			if v := os.Getenv("SORACOM_VERBOSE"); v != "" {
				fmt.Printf("--- WireGuard configuration ----------------------\n%s\n--- End of WireGuard configuration ---------------\n", Config)
			}

			soratun.Up(ctx, Config)
		},
	}
}

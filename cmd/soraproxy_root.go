package cmd

import (
	"github.com/spf13/cobra"
)

// SoraProxyRootCmd defines soraproxy, a top level command.
var SoraProxyRootCmd = &cobra.Command{
	Use:   "soraproxy [command]",
	Short: "soraproxy -- SORACOM Unified Endpoint proxy via SORACOM Arc",
}

func init() {
	SoraProxyRootCmd.PersistentFlags().StringVar(&configPath, "config", "arc.json", "Specify path to SORACOM Arc client configuration file")
	SoraProxyRootCmd.AddCommand(soraProxyUpCmd())
}

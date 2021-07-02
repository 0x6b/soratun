package cmd

import (
	"fmt"
	"github.com/Baozisoftware/qrcode-terminal-go"
	"github.com/spf13/cobra"
	"os"
)

var qrcode bool

func dumpWireGuardConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "wg-config",
		Short:  "Dump soratun configuration file as WireGuard format",
		Args:   cobra.NoArgs,
		PreRun: initSoratun,
		Run: func(cmd *cobra.Command, args []string) {
			c := Config.String()

			if qrcode {
				qrcodeTerminal.New().Get(c).Print()
				os.Exit(0)
			}

			fmt.Println(c)
		},
	}

	cmd.Flags().BoolVar(&qrcode, "barcode", false, "Print WireGuard configuration to the terminal as 2D barcode")

	return cmd
}

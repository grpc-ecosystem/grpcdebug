package cmd

import (
	"fmt"

	"github.com/grpc-ecosystem/grpcdebug/cmd/transport"
	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health [service names]",
	Short: "Check health status of the target service (default \"\").",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			fmt.Println(transport.GetHealthStatus(""))
			return nil
		}
		for _, service := range args {
			fmt.Fprintf(
				w, "%v:\t%v\t\n",
				service,
				transport.GetHealthStatus(service),
			)
		}
		w.Flush()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(healthCmd)
}

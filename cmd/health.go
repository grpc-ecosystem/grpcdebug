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
		// If there are multiple queries, print as table.
		var services []string
		services = append(services, "")
		for _, arg := range args {
			if arg != "" {
				services = append(services, arg)
			}
		}
		for _, service := range services {
			var service_name string
			if service == "" {
				service_name = "<Overall>"
			} else {
				service_name = service
			}
			fmt.Fprintf(
				w, "%v:\t%v\t\n",
				service_name,
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

package cmd

import (
	"fmt"
	"sort"

	"github.com/grpc-ecosystem/grpcdebug/cmd/transport"
	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health [service names]",
	Short: "Check health status of the target service (default \"\").",
	RunE: func(cmd *cobra.Command, args []string) error {
		var services []string
		// Ensure there's the overall health status
		services = append(services, "")
		services = append(services, args...)
		// Sort alphabetically, and deduplicate inputs
		sort.Strings(services)
		j := 0
		for i := 1; i < len(services); i++ {
			if services[i] == services[j] {
				continue
			}
			j++
			services[j] = services[i]
		}
		services = services[:j+1]
		// Print as table
		for _, service := range services {
			var serviceName string
			if service == "" {
				serviceName = "<Overall>"
			} else {
				serviceName = service
			}
			fmt.Fprintf(
				w, "%v:\t%v\t\n",
				serviceName,
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

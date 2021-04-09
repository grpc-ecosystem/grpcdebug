// Defines the root command and global flags

package cmd

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"github.com/grpc-ecosystem/grpcdebug/cmd/config"
	"github.com/grpc-ecosystem/grpcdebug/cmd/transport"
	"github.com/grpc-ecosystem/grpcdebug/cmd/verbose"

	"github.com/spf13/cobra"
)

var verboseFlag, timestampFlag bool
var address, security, credFile, serverNameOverride string

// The table formater
var w = tabwriter.NewWriter(os.Stdout, 10, 0, 3, ' ', 0)

var rootUsageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  grpcdebug <target address> [flags] {{ .CommandPath | ChildCommandPath }} <command>{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "grpcdebug <target address> {{ .CommandPath | ChildCommandPath }} [command] --help" for more information about a command.{{end}}
`

var rootCmd = &cobra.Command{
	Use:   "grpcdebug",
	Short: "grpcdebug is an gRPC service admin CLI",
}

func initConfig() {
	if verboseFlag {
		verbose.EnableDebugOutput()
	}
	c := config.GetServerConfig(address)
	if credFile != "" {
		c.CredentialFile = credFile
	}
	if serverNameOverride != "" {
		c.ServerNameOverride = serverNameOverride
	}
	if security == "tls" {
		c.Security = config.TypeTLS
		if c.CredentialFile == "" {
			rootCmd.Usage()
			log.Fatalf("Please specify credential file under [tls] mode.")
		}
	} else if security != "insecure" {
		rootCmd.Usage()
		log.Fatalf("Unrecognized security mode: %v", security)
	}
	transport.Connect(c)
}

// ChildCommandPath used in template
func ChildCommandPath(path string) string {
	if len(path) <= 10 {
		return ""
	}
	return path[10:]
}

func init() {
	cobra.AddTemplateFunc("ChildCommandPath", ChildCommandPath)
	cobra.OnInitialize(initConfig)
	rootCmd.SetUsageTemplate(rootUsageTemplate)

	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Print verbose information for debugging")
	rootCmd.PersistentFlags().BoolVarP(&timestampFlag, "timestamp", "t", false, "Print timestamp as RFC3339 instead of human readable strings")
	rootCmd.PersistentFlags().StringVar(&security, "security", "insecure", "Defines the type of credentials to use [tls, google-default, insecure]")
	rootCmd.PersistentFlags().StringVar(&credFile, "credential_file", "", "Sets the path of the credential file; used in [tls] mode")
	rootCmd.PersistentFlags().StringVar(&serverNameOverride, "server_name_override", "", "Overrides the peer server name if non empty; used in [tls] mode")
}

// Execute executes the root command.
func Execute() {
	if len(os.Args) > 1 {
		address = os.Args[1]
		os.Args = os.Args[1:]
	} else {
		rootCmd.Usage()
		os.Exit(1)
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

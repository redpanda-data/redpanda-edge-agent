package main

import (
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:     "agent {start|stop|restart}",
		Aliases: []string{"iot"},
		Short:   "Redpanda agent for forwarding events from the edge",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	root.AddCommand(
	// Start
	// Stop
	// Restart
	)
}

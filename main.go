package main

import (
	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:     "agent {start|stop|restart}",
		Aliases: []string{"edge"},
		Short:   "Redpanda agent for forwarding events from the edge",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	root.AddCommand(
		startCommand(),
		stopCommand(),
		restartCommand(),
	)
}

func startCommand() *cobra.Command {
	start := &cobra.Command{
		Use:   "start",
		Short: "Start the Redpanda forwarding agent",
		Run: func(cmd *cobra.Command, args []string) {
			log.Infof("Starting Redpanda forwarding agent")
		},
	}
	return start
}

func stopCommand() *cobra.Command {
	stop := &cobra.Command{
		Use:   "stop",
		Short: "Stop the Redpanda forwarding agent",
		Run: func(cmd *cobra.Command, args []string) {
			log.Infof("Stopping Redpanda forwarding agent")
		},
	}
	return stop
}

func restartCommand() *cobra.Command {
	restart := &cobra.Command{
		Use:   "stop",
		Short: "Stop the Redpanda forwarding agent",
		Run: func(cmd *cobra.Command, args []string) {
			stopCommand()
			startCommand()
		},
	}
	return restart
}

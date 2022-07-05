package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var binaryName = "redpanda-edge-agent"
var defaultConfigPath = func() string {
	return filepath.Join("/etc/redpanda", "conf", "agent.yaml")
}()
var pidFile = func() string {
	exPath, err := os.Executable()
	check(err)
	exDir := filepath.Dir(exPath)
	return filepath.Join(exDir, "agent.pid")
}()

type Params struct {
	ConfigPath       string
	LogLevel         string
	DisableNameCheck bool
}

type pluginHelp struct {
	Path    string   `json:"path,omitempty"`
	Short   string   `json:"short,omitempty"`
	Long    string   `json:"long,omitempty"`
	Example string   `json:"example,omitempty"`
	Args    []string `json:"args,omitempty"`
}

// In support of rpk.ac --help-autocomplete.
func traverseHelp(cmd *cobra.Command, pieces []string) []pluginHelp {
	pieces = append(pieces, strings.Split(cmd.Use, " ")[0])
	help := []pluginHelp{{
		Path:    strings.Join(pieces, "_"),
		Short:   cmd.Short,
		Long:    cmd.Long,
		Example: cmd.Example,
		Args:    cmd.ValidArgs,
	}}
	for _, cmd := range cmd.Commands() {
		help = append(help, traverseHelp(cmd, pieces)...)
	}
	return help
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	var helpAutocomplete bool
	params := new(Params)

	root := &cobra.Command{
		Use:     "agent {start|stop|restart}",
		Aliases: []string{"edge"},
		Short:   "Redpanda agent for forwarding events from the edge.",
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		PersistentPreRun: func(root *cobra.Command, _ []string) {
			if helpAutocomplete {
				json.NewEncoder(os.Stdout).Encode(traverseHelp(root, nil))
				os.Exit(0)
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	root.AddCommand(
		startCommand(params),
		stopCommand(params),
		restartCommand(params),
	)

	root.PersistentFlags().StringVarP(&params.ConfigPath, "config", "c", defaultConfigPath, "path to agent config file")
	root.PersistentFlags().StringVarP(&params.LogLevel, "loglevel", "l", "info", "logging level")
	root.PersistentFlags().BoolVar(&helpAutocomplete, "help-autocomplete", false, "autocompletion help for rpk")
	root.PersistentFlags().MarkHidden("help-autocomplete")

	err := root.Execute()
	check(err)
}

func startCommand(params *Params) *cobra.Command {
	start := &cobra.Command{
		Use:   "start",
		Short: "Start the agent",
		Run: func(cmd *cobra.Command, args []string) {
			p := exec.Command(binaryName,
				"-config", params.ConfigPath,
				"-loglevel", params.LogLevel,
				"-enablelog", "true",
			)
			fmt.Fprintf(os.Stderr, "Running command: %s \n", p.String())

			err := p.Start()
			check(err)
			fmt.Fprintf(os.Stderr, "Started agent: %d \n", p.Process.Pid)

			// Write pid to file for convenience
			pidStr := strconv.Itoa(p.Process.Pid)
			err = os.WriteFile(pidFile, []byte(pidStr), 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Unable to save pid: %s \n", err.Error())
			}
		},
	}
	return start
}

func checkProcessName(pid int) bool {
	if runtime.GOOS == "linux" {
		statusFile := fmt.Sprintf("/proc/%d/status", pid)
		_, err := os.Stat(statusFile)
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "Unable to verify process name. Status file does not exist: %s \n", statusFile)
			return false
		}
		statusBytes, err := os.ReadFile(statusFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to verify process name. Error reading status file: %s \n", statusFile)
			return false
		}
		lines := strings.Split(string(statusBytes), "\n")
		for _, line := range lines {
			if !strings.Contains(line, ":") {
				continue
			}
			kv := strings.SplitN(line, ":", 2)
			k := string(strings.TrimSpace(kv[0]))
			v := string(strings.TrimSpace(kv[1]))
			if k == "Name" && v == binaryName {
				fmt.Fprintf(os.Stderr, "Verified process name for pid %d is %s \n", pid, binaryName)
				return true
			}
		}
	}
	return false
}

func stopCommand(params *Params) *cobra.Command {
	stop := &cobra.Command{
		Use:   "stop",
		Short: "Stop the agent",
		Run: func(cmd *cobra.Command, args []string) {
			// Start by terminating the latest pid, if the pid file exists
			pidBytes, _ := os.ReadFile(pidFile)
			if pidBytes != nil {
				pid, _ := strconv.Atoi(string(pidBytes))
				if !params.DisableNameCheck && !checkProcessName(pid) {
					fmt.Fprintf(os.Stderr, "Unable to verify process name for pid: %d \n", pid)
					return
				}
				p, err := os.FindProcess(pid)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
				p.Signal(os.Interrupt) // SIGINT
				fmt.Fprintf(os.Stderr, "Interrupted agent with pid: %d \n", pid)
				err = os.Remove(pidFile)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Unable to remove pid file: %s \n", pidFile)
				}
			}
		},
	}
	stop.PersistentFlags().BoolVarP(&params.DisableNameCheck, "disable-check", "d", false,
		"disable process name check for pid on stop (set for darwin and windows)")
	return stop
}

func restartCommand(params *Params) *cobra.Command {
	restart := &cobra.Command{
		Use:   "restart",
		Short: "Restart the agent",
		Run: func(cmd *cobra.Command, args []string) {
			stopCommand(params)
			startCommand(params)
		},
	}
	return restart
}

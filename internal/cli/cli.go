// Package cli implements the volok command line interface.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
)

// version is injected at build time via -ldflags "-X volok/internal/cli.version=<hash>".
var version string

const (
	defaultFile  = "/etc/volok/volok.json"
	versionValue = "dev"

	subShow      = "show"
	initSystemd  = "systemd"
	initProcd    = "procd"
	formatBase64 = "base64"
)

// App is the CLI entry point.
type App struct {
	name   string
	args   []string
	file   string
	stdout io.Writer
	stderr io.Writer
}

// New creates a CLI app.
func New(name string, args []string, stdout, stderr io.Writer) *App {
	return &App{name: name, args: args, stdout: stdout, stderr: stderr}
}

// Run dispatches the command and returns a process exit code.
func (a *App) Run() int {
	if len(a.args) == 0 {
		a.printHelp()
		return 1
	}

	fs := flag.NewFlagSet("volok", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	fs.StringVar(&a.file, "file", "", "path to volok.json (default $VOLOK_FILE or /etc/volok/volok.json)")
	if err := fs.Parse(a.args); err != nil {
		return 1
	}

	rest := fs.Args()
	if len(rest) == 0 {
		a.printHelp()
		return 1
	}
	if a.file == "" {
		a.file = os.Getenv("VOLOK_FILE")
	}
	if a.file == "" {
		a.file = defaultFile
	}

	cmd := rest[0]
	if cmd == "help" || cmd == "-h" || cmd == "--help" {
		a.printHelp()
		return 0
	}
	if cmd == "version" {
		fmt.Fprintf(a.stdout, "volok version %s\n", currentVersion())
		return 0
	}

	handler, ok := commands[cmd]
	if !ok {
		fmt.Fprintf(a.stderr, "unknown command %q\n", cmd)
		a.printHelp()
		return 1
	}
	if err := handler(a, rest[1:]); err != nil {
		fmt.Fprintf(a.stderr, "%s: %v\n", cmd, err)
		return 1
	}
	return 0
}

func currentVersion() string {
	if version != "" {
		return version
	}
	return versionValue
}

func (a *App) printHelp() {
	fmt.Fprintf(a.stdout, `%s - minimal VLESS node library

Usage: %s [--file PATH] <command> [args]

Commands:
  init                     create volok.json with a fresh admin token
  serve                    run the HTTP server in foreground
  config show|validate     show redacted config or validate it
  config set <key> <value> change listen or public-url
  token show|rotate        show or rotate the admin token
  user add|list|remove     manage reader subscription tokens
  node list|show|add|rename|enable|disable|remove
  subscription url|export  build a subscription URL or export links
  install-command          print the curl|bash command for a VPS
  service <subcommand>     install/start/stop/restart/status/enable/disable
  version                  print version
  help                     show this help

Use "volok <command> -h" for command-specific flags.
`, a.name, a.name)
}

type handlerFunc func(a *App, args []string) error

var commands = map[string]handlerFunc{
	"init":            runInit,
	"serve":           runServe,
	"config":          runConfig,
	"token":           runToken,
	"user":            runUser,
	"node":            runNode,
	"subscription":    runSubscription,
	"install-command": runInstallCommand,
	"service":         runService,
}

func requireArgs(args []string, n int) error {
	if len(args) < n {
		return fmt.Errorf("missing arguments")
	}
	return nil
}

func flagSet(name string, out io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(out)
	return fs
}

func unknownArg(args []string, rest string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected argument %q for %s", args[0], rest)
	}
	return nil
}

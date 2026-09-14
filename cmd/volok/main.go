// Command volok is a minimal VLESS node library and subscription server.
//
// It stores direct VLESS links in a single JSON file, serves them over HTTP
// to reader tokens, and ships a VPS installer that registers new nodes.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/quonaro/lota/engine"

	"volok/internal/cli"
)

func main() {
	args, file := extractFileFlag(os.Args[1:])
	cli.SetConfigFile(file)

	app, err := cli.BuildCLI(os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		app.PrintHelp()
		if len(args) == 0 {
			os.Exit(1)
		}
		return
	}

	if err := app.Run(context.Background(), args); err != nil {
		var groupErr *engine.GroupError
		if errors.As(err, &groupErr) {
			app.PrintGroupHelp(groupErr.Groups)
			os.Exit(1)
		}
		color.New(color.FgRed).Fprintf(os.Stderr, "run: %v\n", err)
		os.Exit(1)
	}
}

// extractFileFlag removes --file=path and --file path from args, returning
// the remaining arguments and the extracted path. Lota does not parse
// global flags, so --file is handled here.
func extractFileFlag(args []string) ([]string, string) {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--file" || arg == "-f" {
			if i+1 < len(args) {
				i++
				return append(out, args[i+1:]...), args[i]
			}
			continue
		}
		if len(arg) > len("--file=") && arg[:len("--file=")] == "--file=" {
			return append(out, args[i+1:]...), arg[len("--file="):]
		}
		out = append(out, arg)
	}
	return out, ""
}

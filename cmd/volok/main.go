// Command volok is a minimal VLESS node library and subscription server.
//
// It stores direct VLESS links in a single JSON file, serves them over HTTP
// to reader tokens, and ships a VPS installer that registers new nodes.
package main

import (
	"os"

	"volok/internal/cli"
)

func main() {
	app := cli.New(os.Args[0], os.Args[1:], os.Stdout, os.Stderr)
	code := app.Run()
	os.Exit(code)
}

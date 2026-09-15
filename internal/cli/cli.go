// Package cli implements the volok command line interface on top of the
// Lota engine (github.com/quonaro/lota). Command definitions live in
// cli.yml; native handlers are registered here.
package cli

import (
	_ "embed"
	"io"
	"os"

	"github.com/quonaro/lota/engine"
)

const defaultFile = "/etc/volok/volok.json"

const (
	initSystemd  = "systemd"
	initProcd    = "procd"
	formatBase64 = "base64"
	strTrue      = "true"
)

// configFile is set by main from the --file flag (removed from args before
// they reach Lota, which does not parse global flags).
var configFile string

// SetConfigFile overrides the volok.json path from the --file flag.
func SetConfigFile(path string) {
	configFile = path
}

// filePath resolves the JSON path: flag, then VOLOK_FILE, then the default.
func filePath() string {
	if configFile != "" {
		return configFile
	}
	if p := os.Getenv("VOLOK_FILE"); p != "" {
		return p
	}
	return defaultFile
}

//go:embed cli.yml
var cliYAML []byte

// BuildCLI constructs the Lota app with all native handlers registered.
func BuildCLI(stdout, stderr io.Writer) (*engine.App, error) {
	builder := engine.NewBuilder("volok", cliYAML)

	register := map[string]engine.NativeFunc{
		"init":         runInit,
		"serve":        runServe,
		"token.show":   runTokenShow,
		"token.rotate": runTokenRotate,
		"user.add":     runUserAdd,
		"user.list":    runUserList,
		"user.remove":  runUserRemove,
		"node.list":    runNodeList,
		"node.show":    runNodeShow,
		"node.add":     runNodeAdd,
		"node.rename":  runNodeRename,
		"node.enable":  runNodeEnable,
		"node.disable": runNodeDisable,
		"node.remove":  runNodeRemove,
		"proxy.init":   runProxyInit,
		"proxy.show":   runProxyShow,
		"proxy.config": runProxyConfig,
		"proxy.remove": runProxyRemove,
	}
	for path, fn := range register {
		builder.RegisterNative(path, fn)
	}

	return builder.WithOptions(engine.Options{
		Stdout: stdout,
		Stderr: stderr,
	}).Build()
}

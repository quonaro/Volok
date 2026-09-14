package cli

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

//go:embed assets/volok.service
var systemdUnit []byte

//go:embed assets/volok.init
var procdInit []byte

const (
	systemdPath = "/etc/systemd/system/volok.service"
	procdPath   = "/etc/init.d/volok"
)

// execCommand is overridable in tests.
var execCommand = func(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func runService(a *App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("service requires a subcommand: install|start|stop|restart|status|enable|disable")
	}
	action := args[0]
	switch action {
	case "install":
		return serviceInstall(a, args[1:])
	case "start", "stop", "restart", "status", "enable", "disable":
		return serviceControl(a, action)
	default:
		return fmt.Errorf("unknown service subcommand %q", action)
	}
}

func serviceInstall(a *App, args []string) error {
	if err := unknownArg(args, "service install"); err != nil {
		return err
	}
	switch detectInit() {
	case initSystemd:
		if err := writeIfOurs(systemdPath, systemdUnit, 0o644); err != nil {
			return err
		}
		if _, err := execCommand("systemctl", "daemon-reload"); err != nil {
			return fmt.Errorf("systemctl daemon-reload: %w", err)
		}
		if _, err := execCommand("systemctl", "enable", "volok"); err != nil {
			return fmt.Errorf("systemctl enable: %w", err)
		}
		fmt.Fprintln(a.stdout, "installed systemd unit /etc/systemd/system/volok.service")
	case initProcd:
		if err := writeIfOurs(procdPath, procdInit, 0o755); err != nil {
			return err
		}
		if _, err := execCommand(procdPath, "enable"); err != nil {
			return fmt.Errorf("enabling volok init script: %w", err)
		}
		fmt.Fprintln(a.stdout, "installed OpenWrt init script /etc/init.d/volok")
	default:
		return fmt.Errorf("no supported init system found (systemd or procd)")
	}
	fmt.Fprintln(a.stdout, "create the library first with: volok init --public-url https://…")
	return nil
}

func serviceControl(a *App, action string) error {
	switch detectInit() {
	case initSystemd:
		out, err := execCommand("systemctl", action, "volok")
		if err != nil {
			return fmt.Errorf("systemctl %s volok: %w (%s)", action, err, out)
		}
		if out != "" {
			fmt.Fprintln(a.stdout, out)
		}
	case initProcd:
		out, err := execCommand(procdPath, action)
		if err != nil {
			return fmt.Errorf("%s %s: %w (%s)", procdPath, action, err, out)
		}
		if out != "" {
			fmt.Fprintln(a.stdout, out)
		}
	default:
		return fmt.Errorf("no supported init system found (systemd or procd)")
	}
	return nil
}

func detectInit() string {
	if _, err := os.Stat("/etc/rc.common"); err == nil {
		return initProcd
	}
	if _, err := exec.LookPath("systemctl"); err == nil {
		return initSystemd
	}
	return ""
}

// writeIfOurs writes content unless an existing file differs from it.
func writeIfOurs(path string, content []byte, mode os.FileMode) error {
	if existing, err := os.ReadFile(path); err == nil {
		if string(existing) != string(content) {
			return fmt.Errorf("%s already exists with different content; refusing to overwrite", path)
		}
		return nil
	}
	return os.WriteFile(path, content, mode)
}

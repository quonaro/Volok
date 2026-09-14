package cli

import (
	"encoding/base64"
	"fmt"
	"os"

	"volok/internal/store"
	"volok/internal/subscription"
)

func runSubscription(a *App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("subscription requires a subcommand: url|export")
	}
	switch args[0] {
	case "url":
		return subscriptionURL(a, args[1:])
	case "export":
		return subscriptionExport(a, args[1:])
	default:
		return fmt.Errorf("unknown subscription subcommand %q", args[0])
	}
}

func subscriptionURL(a *App, args []string) error {
	fs := flagSet("subscription url", a.stderr)
	format := fs.String("format", "plain", "plain or base64")
	token := fs.String("user-token", "", "reader token from users")
	tokenStdin := fs.Bool("token-stdin", false, "read the reader token from stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := unknownArg(fs.Args(), "subscription url"); err != nil {
		return err
	}
	if *format != "plain" && *format != formatBase64 {
		return fmt.Errorf("format must be plain or base64")
	}
	reader := *token
	if *tokenStdin {
		line, err := readLine(stdinReader())
		if err != nil {
			return fmt.Errorf("reading token from stdin: %w", err)
		}
		reader = line
	}
	if reader == "" {
		return fmt.Errorf("--user-token or --token-stdin is required")
	}

	s := store.Open(a.file)
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	if !cfg.HasUser(reader) {
		return fmt.Errorf("token is not a registered user token")
	}
	url := fmt.Sprintf("%s/sub?token=%s", cfg.PublicURL, reader)
	if *format == formatBase64 {
		url += "&format=base64"
	}
	fmt.Fprintln(a.stdout, url)
	return nil
}

func subscriptionExport(a *App, args []string) error {
	fs := flagSet("subscription export", a.stderr)
	format := fs.String("format", "plain", "plain or base64")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := unknownArg(fs.Args(), "subscription export"); err != nil {
		return err
	}
	if *format != "plain" && *format != formatBase64 {
		return fmt.Errorf("format must be plain or base64")
	}

	s := store.Open(a.file)
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	body := subscription.Build(cfg)
	if *format == formatBase64 {
		body = base64.StdEncoding.EncodeToString([]byte(body))
	}
	fmt.Fprint(a.stdout, body)
	if len(body) == 0 || body[len(body)-1] != '\n' {
		fmt.Fprintln(a.stdout)
	}
	return nil
}

func runInstallCommand(a *App, args []string) error {
	if err := unknownArg(args, "install-command"); err != nil {
		return err
	}
	s := store.Open(a.file)
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "curl -fsS --proto '=https' '%s/register?token=%s' | bash\n", cfg.PublicURL, cfg.Token)
	return nil
}

// stdinReader returns stdin for token reading helpers.
func stdinReader() *os.File { return os.Stdin }

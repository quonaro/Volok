package cli

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"volok/internal/httpserver"
	"volok/internal/store"
)

func runInit(a *App, args []string) error {
	fs := flagSet("init", a.stderr)
	publicURL := fs.String("public-url", "", "public https origin of Volok")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := unknownArg(fs.Args(), "init"); err != nil {
		return err
	}
	if *publicURL == "" {
		return fmt.Errorf("--public-url is required (e.g. https://vpn.example.com)")
	}
	clean, err := store.CleanPublicURL(*publicURL)
	if err != nil {
		return err
	}
	s := store.Open(a.file)
	if s.Exists() {
		return fmt.Errorf("refusing to overwrite existing %s", a.file)
	}
	cfg, err := s.Init(clean)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "created %s\n", a.file)
	fmt.Fprintf(a.stdout, "admin token: %s\n", cfg.Token)
	fmt.Fprintf(a.stdout, "keep this token secret; it grants installer and registration access\n")
	return nil
}

func runServe(a *App, args []string) error {
	if err := unknownArg(args, "serve"); err != nil {
		return err
	}
	s := store.Open(a.file)
	cfg, err := s.Read()
	if err != nil {
		return err
	}

	addr := cfg.Listen
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	defer ln.Close()

	srv := httpserver.New(s)
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	fmt.Fprintf(a.stdout, "volok serving on %s\n", addr)
	select {
	case <-ctx.Done():
		fmt.Fprintln(a.stdout, "shutting down")
		return nil
	case err := <-serveErr:
		return err
	}
}

func runConfig(a *App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("config requires a subcommand: show|validate|set")
	}
	switch args[0] {
	case subShow:
		return configShow(a, args[1:])
	case "validate":
		return configValidate(a, args[1:])
	case "set":
		return configSet(a, args[1:])
	default:
		return fmt.Errorf("unknown config subcommand %q", args[0])
	}
}

func configShow(a *App, args []string) error {
	if err := unknownArg(args, "config show"); err != nil {
		return err
	}
	s := store.Open(a.file)
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "file:            %s\n", a.file)
	fmt.Fprintf(a.stdout, "schema_version:  %d\n", cfg.SchemaVersion)
	fmt.Fprintf(a.stdout, "listen:          %s\n", cfg.Listen)
	fmt.Fprintf(a.stdout, "public_url:      %s\n", cfg.PublicURL)
	fmt.Fprintf(a.stdout, "admin token:     <redacted>\n")
	fmt.Fprintf(a.stdout, "users:           %d token(s)\n", len(cfg.Users))
	fmt.Fprintf(a.stdout, "nodes:           %d node(s)\n", len(cfg.Nodes))
	return nil
}

func configValidate(a *App, args []string) error {
	if err := unknownArg(args, "config validate"); err != nil {
		return err
	}
	s := store.Open(a.file)
	if _, err := s.Read(); err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "%s is valid\n", a.file)
	return nil
}

func configSet(a *App, args []string) error {
	if err := requireArgs(args, 2); err != nil {
		return err
	}
	key, value := args[0], strings.Join(args[1:], " ")
	s := store.Open(a.file)
	_, err := s.Update(func(c *store.Config) error {
		switch key {
		case "listen":
			if _, _, err := net.SplitHostPort(value); err != nil {
				return fmt.Errorf("invalid listen address %q", value)
			}
			c.Listen = value
		case "public-url":
			clean, err := store.CleanPublicURL(value)
			if err != nil {
				return err
			}
			c.PublicURL = clean
		default:
			return fmt.Errorf("unknown config key %q (allowed: listen, public-url)", key)
		}
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "set %s\n", key)
	if key == "listen" {
		fmt.Fprintln(a.stdout, "restart the service for the new listen address to take effect")
	}
	return nil
}

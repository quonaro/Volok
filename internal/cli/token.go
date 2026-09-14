package cli

import (
	"fmt"

	"volok/internal/store"
)

func runToken(a *App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("token requires a subcommand: show|rotate")
	}
	switch args[0] {
	case subShow:
		return tokenShow(a, args[1:])
	case "rotate":
		return tokenRotate(a, args[1:])
	default:
		return fmt.Errorf("unknown token subcommand %q", args[0])
	}
}

func tokenShow(a *App, args []string) error {
	if err := unknownArg(args, "token show"); err != nil {
		return err
	}
	s := store.Open(a.file)
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "%s\n", cfg.Token)
	return nil
}

func tokenRotate(a *App, args []string) error {
	fs := flagSet("token rotate", a.stderr)
	yes := fs.Bool("yes", false, "confirm rotation")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := unknownArg(fs.Args(), "token rotate"); err != nil {
		return err
	}
	if !*yes {
		return fmt.Errorf("rotation requires --yes; existing installer URLs and in-flight callbacks will stop working")
	}

	s := store.Open(a.file)
	cfg, err := s.Update(func(c *store.Config) error {
		next, err := store.NewToken()
		if err != nil {
			return err
		}
		c.Token = next
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "admin token rotated: %s\n", cfg.Token)
	fmt.Fprintln(a.stdout, "update your installer command and any stored callbacks")
	return nil
}

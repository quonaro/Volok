package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"volok/internal/store"
)

func runUser(a *App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("user requires a subcommand: add|list|remove")
	}
	switch args[0] {
	case "add":
		return userAdd(a, args[1:])
	case "list":
		return userList(a, args[1:])
	case "remove":
		return userRemove(a, args[1:])
	default:
		return fmt.Errorf("unknown user subcommand %q", args[0])
	}
}

func userAdd(a *App, args []string) error {
	if err := unknownArg(args, "user add"); err != nil {
		return err
	}
	s := store.Open(a.file)
	cfg, err := s.Update(func(c *store.Config) error {
		token, err := store.NewToken()
		if err != nil {
			return err
		}
		c.Users = append(c.Users, token)
		return nil
	})
	if err != nil {
		return err
	}
	newToken := cfg.Users[len(cfg.Users)-1]
	fmt.Fprintf(a.stdout, "user token: %s\n", newToken)
	fmt.Fprintf(a.stdout, "subscription: %s/sub?token=%s\n", cfg.PublicURL, newToken)
	return nil
}

func userList(a *App, args []string) error {
	fs := flagSet("user list", a.stderr)
	showTokens := fs.Bool("show-tokens", false, "print full tokens instead of fingerprints")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := unknownArg(fs.Args(), "user list"); err != nil {
		return err
	}

	s := store.Open(a.file)
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	if len(cfg.Users) == 0 {
		fmt.Fprintln(a.stdout, "no user tokens")
		return nil
	}
	for i, token := range cfg.Users {
		if *showTokens {
			fmt.Fprintf(a.stdout, "%d\t%s\n", i, token)
		} else {
			fmt.Fprintf(a.stdout, "%d\t%s\n", i, fingerprint(token))
		}
	}
	return nil
}

func userRemove(a *App, args []string) error {
	fs := flagSet("user remove", a.stderr)
	yes := fs.Bool("yes", false, "confirm removal")
	tokenStdin := fs.Bool("token-stdin", false, "read the token from stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*yes {
		return fmt.Errorf("removal requires --yes")
	}
	token := ""
	if *tokenStdin {
		line, err := readLine(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading token from stdin: %w", err)
		}
		token = line
	} else {
		rest := fs.Args()
		if len(rest) != 1 {
			return fmt.Errorf("expected exactly one token argument or --token-stdin")
		}
		token = rest[0]
	}

	s := store.Open(a.file)
	removed := false
	_, err := s.Update(func(c *store.Config) error {
		out := c.Users[:0]
		for _, u := range c.Users {
			if u == token {
				removed = true
				continue
			}
			out = append(out, u)
		}
		c.Users = out
		return nil
	})
	if err != nil {
		return err
	}
	if !removed {
		return fmt.Errorf("no matching user token found")
	}
	fmt.Fprintln(a.stdout, "user token removed")
	fmt.Fprintln(a.stdout, "note: this stops future subscription updates, it does not revoke already distributed VLESS links")
	return nil
}

func fingerprint(token string) string {
	if len(token) < 12 {
		return "<invalid>"
	}
	return token[:6] + "…" + token[len(token)-6:]
}

func readLine(r io.Reader) (string, error) {
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

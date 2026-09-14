package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/quonaro/lota/engine"

	"volok/internal/store"
)

func runUserAdd(_ context.Context, nctx engine.NativeContext) error {
	s := store.Open(filePath())
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
	fmt.Fprint(nctx.Stdout, "user token: ")
	yellow(nctx.Stdout, "%s\n", newToken)
	fmt.Fprint(nctx.Stdout, "subscription: ")
	cyan(nctx.Stdout, "%s/sub?token=%s\n", cfg.PublicURL, newToken)
	return nil
}

func runUserList(_ context.Context, nctx engine.NativeContext) error {
	showTokens := nctx.Args["show-tokens"] == strTrue
	s := store.Open(filePath())
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	if len(cfg.Users) == 0 {
		fmt.Fprintln(nctx.Stdout, "no user tokens")
		return nil
	}
	for i, token := range cfg.Users {
		if showTokens {
			fmt.Fprintf(nctx.Stdout, "%d\t%s\n", i, token)
		} else {
			fmt.Fprintf(nctx.Stdout, "%d\t%s\n", i, fingerprint(token))
		}
	}
	return nil
}

func runUserRemove(_ context.Context, nctx engine.NativeContext) error {
	if nctx.Args["yes"] != strTrue {
		return fmt.Errorf("removal requires --yes")
	}
	token := nctx.Args["token"]
	if nctx.Args["token-stdin"] == strTrue {
		line, err := readLine(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading token from stdin: %w", err)
		}
		token = line
	}
	if token == "" {
		return fmt.Errorf("expected a token argument or --token-stdin")
	}

	s := store.Open(filePath())
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
	fmt.Fprintln(nctx.Stdout, "user token removed")
	fmt.Fprintln(nctx.Stdout, "note: this stops future subscription updates, it does not revoke already distributed VLESS links")
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

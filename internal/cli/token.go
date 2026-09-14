package cli

import (
	"context"
	"fmt"

	"github.com/quonaro/lota/engine"

	"volok/internal/store"
)

func runTokenShow(_ context.Context, nctx engine.NativeContext) error {
	s := store.Open(filePath())
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	fmt.Fprintf(nctx.Stdout, "%s\n", cfg.Token)
	return nil
}

func runTokenRotate(_ context.Context, nctx engine.NativeContext) error {
	if nctx.Args["yes"] != strTrue {
		return fmt.Errorf("rotation requires --yes; existing installer URLs and in-flight callbacks will stop working")
	}
	s := store.Open(filePath())
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
	fmt.Fprintf(nctx.Stdout, "admin token rotated: %s\n", cfg.Token)
	fmt.Fprintln(nctx.Stdout, "update your installer command and any stored callbacks")
	return nil
}

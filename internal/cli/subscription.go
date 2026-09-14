package cli

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/quonaro/lota/engine"

	"volok/internal/store"
	"volok/internal/subscription"
)

func runSubscriptionURL(_ context.Context, nctx engine.NativeContext) error {
	format := nctx.Args["format"]
	if format != "plain" && format != formatBase64 {
		return fmt.Errorf("format must be plain or base64")
	}
	reader := nctx.Args["user-token"]
	if nctx.Args["token-stdin"] == strTrue {
		line, err := readLine(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading token from stdin: %w", err)
		}
		reader = line
	}
	if reader == "" {
		return fmt.Errorf("--user-token or --token-stdin is required")
	}

	s := store.Open(filePath())
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	if !cfg.HasUser(reader) {
		return fmt.Errorf("token is not a registered user token")
	}
	url := fmt.Sprintf("%s/sub?token=%s", cfg.PublicURL, reader)
	if format == formatBase64 {
		url += "&format=base64"
	}
	fmt.Fprintln(nctx.Stdout, url)
	return nil
}

func runSubscriptionExport(_ context.Context, nctx engine.NativeContext) error {
	format := nctx.Args["format"]
	if format != "plain" && format != formatBase64 {
		return fmt.Errorf("format must be plain or base64")
	}
	s := store.Open(filePath())
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	body := subscription.Build(cfg)
	if format == formatBase64 {
		body = base64.StdEncoding.EncodeToString([]byte(body))
	}
	fmt.Fprint(nctx.Stdout, body)
	if len(body) == 0 || body[len(body)-1] != '\n' {
		fmt.Fprintln(nctx.Stdout)
	}
	return nil
}

func runInstallCommand(_ context.Context, nctx engine.NativeContext) error {
	s := store.Open(filePath())
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	fmt.Fprintf(nctx.Stdout, "curl -fsS --proto '=https' '%s/register?token=%s' | bash\n", cfg.PublicURL, cfg.Token)
	return nil
}

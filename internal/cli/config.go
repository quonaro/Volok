package cli

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/quonaro/lota/engine"

	"volok/internal/httpserver"
	"volok/internal/proxy"
	"volok/internal/store"
)

func runInit(_ context.Context, nctx engine.NativeContext) error {
	publicURL := nctx.Args["public-url"]
	if publicURL == "" {
		return fmt.Errorf("--public-url is required (e.g. https://vpn.example.com)")
	}
	clean, err := store.CleanPublicURL(publicURL)
	if err != nil {
		return err
	}
	s := store.Open(filePath())
	if s.Exists() {
		return fmt.Errorf("refusing to overwrite existing %s", filePath())
	}
	cfg, err := s.Init(clean)
	if err != nil {
		return err
	}
	green(nctx.Stdout, "created %s\n", filePath())
	fmt.Fprint(nctx.Stdout, "admin token: ")
	yellow(nctx.Stdout, "%s\n", cfg.Token)
	fmt.Fprintln(nctx.Stdout, "keep this token secret; it grants installer and registration access")
	return nil
}

func runServe(_ context.Context, nctx engine.NativeContext) error {
	s := store.Open(filePath())
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

	// Start the embedded sing-box proxy when a proxy identity is configured.
	var proxyErr chan error
	if cfg.Proxy != nil {
		proxyErr = make(chan error, 1)
		runner := &proxy.Runner{}
		go func() { proxyErr <- runner.Start(ctx, cfg) }()
		green(nctx.Stdout, "proxy relay on :%d\n", cfg.Proxy.Port)
	}

	green(nctx.Stdout, "volok serving on %s\n", addr)
	select {
	case <-ctx.Done():
		fmt.Fprintln(nctx.Stdout, "shutting down")
		return nil
	case err := <-serveErr:
		return err
	case err := <-proxyErr:
		return err
	}
}

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

	// Auto-create the relay proxy identity on first serve so the router
	// relay starts without a separate setup step.
	if cfg.Proxy == nil {
		_, err = s.Update(func(c *store.Config) error {
			p, perr := store.NewProxy(8443, "www.cloudflare.com")
			if perr != nil {
				return perr
			}
			c.Proxy = p
			return nil
		})
		if err != nil {
			return fmt.Errorf("creating proxy identity: %w", err)
		}
		cfg, err = s.Read()
		if err != nil {
			return err
		}
		green(nctx.Stdout, "proxy identity auto-created on :%d\n", cfg.Proxy.Port)
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
	// A proxy failure is logged but does not stop the HTTP server.
	var proxyErr chan error
	if cfg.Proxy != nil {
		proxyErr = make(chan error, 1)
		runner := &proxy.Runner{}
		go func() {
			if err := runner.Start(ctx, cfg); err != nil {
				proxyErr <- err
			}
		}()
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

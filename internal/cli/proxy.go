package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/quonaro/lota/engine"

	"volok/internal/proxy"
	"volok/internal/store"
)

func runProxyInit(_ context.Context, nctx engine.NativeContext) error {
	portStr := nctx.Args["port"]
	port := 443
	if portStr != "" {
		var err error
		port, err = strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid port %q", portStr)
		}
	}
	sni := nctx.Args["sni"]
	if sni == "" {
		sni = "www.cloudflare.com"
	}

	s := store.Open(filePath())
	cfg, err := s.Update(func(c *store.Config) error {
		if c.Proxy != nil {
			return fmt.Errorf("proxy already initialized; use 'volok proxy remove --yes' first")
		}
		p, err := store.NewProxy(port, sni)
		if err != nil {
			return err
		}
		c.Proxy = p
		return nil
	})
	if err != nil {
		return err
	}
	green(nctx.Stdout, "proxy identity created\n")
	p := cfg.Proxy
	fmt.Fprintf(nctx.Stdout, "uuid:       %s\n", p.UUID)
	fmt.Fprintf(nctx.Stdout, "port:       %d\n", p.Port)
	fmt.Fprintf(nctx.Stdout, "sni:        %s\n", p.SNI)
	fmt.Fprintln(nctx.Stdout, "run 'volok proxy config' to generate the sing-box config")
	return nil
}

func runProxyShow(_ context.Context, nctx engine.NativeContext) error {
	s := store.Open(filePath())
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	if cfg.Proxy == nil {
		red(nctx.Stdout, "proxy not initialized\n")
		fmt.Fprintln(nctx.Stdout, "run 'volok proxy init' to create a proxy identity")
		return nil
	}
	p := cfg.Proxy
	cyan(nctx.Stdout, "proxy identity\n")
	fmt.Fprintf(nctx.Stdout, "uuid:       %s\n", p.UUID)
	fmt.Fprintf(nctx.Stdout, "port:       %d\n", p.Port)
	fmt.Fprintf(nctx.Stdout, "sni:        %s\n", p.SNI)
	fmt.Fprintf(nctx.Stdout, "public_key: %s\n", p.PublicKey)
	fmt.Fprintf(nctx.Stdout, "short_id:   %s\n", p.ShortID)
	return nil
}

func runProxyConfig(_ context.Context, nctx engine.NativeContext) error {
	s := store.Open(filePath())
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	if cfg.Proxy == nil {
		return fmt.Errorf("proxy not initialized; run 'volok proxy init' first")
	}
	out, err := proxy.BuildSingBoxConfig(cfg)
	if err != nil {
		return err
	}
	fmt.Fprint(nctx.Stdout, out)
	return nil
}

func runProxyRemove(_ context.Context, nctx engine.NativeContext) error {
	if nctx.Args["yes"] != strTrue {
		return fmt.Errorf("removal requires --yes")
	}
	s := store.Open(filePath())
	_, err := s.Update(func(c *store.Config) error {
		c.Proxy = nil
		return nil
	})
	if err != nil {
		return err
	}
	green(nctx.Stdout, "proxy identity removed\n")
	return nil
}

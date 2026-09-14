package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/quonaro/lota/engine"

	"volok/internal/store"
	"volok/internal/vless"
)

func runNodeList(_ context.Context, nctx engine.NativeContext) error {
	s := store.Open(filePath())
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	if len(cfg.Nodes) == 0 {
		fmt.Fprintln(nctx.Stdout, "no nodes")
		return nil
	}
	for _, n := range cfg.Nodes {
		p, err := vless.Parse(n.URL)
		if err != nil {
			return fmt.Errorf("node %s: %w", n.ID, err)
		}
		fmt.Fprint(nctx.Stdout, n.ID, "\t")
		cyan(nctx.Stdout, "%s", n.Name)
		fmt.Fprintf(nctx.Stdout, "\t%s:%d\t", p.Host, p.Port)
		if n.Enabled {
			green(nctx.Stdout, "enabled\n")
		} else {
			red(nctx.Stdout, "disabled\n")
		}
	}
	return nil
}

func runNodeShow(_ context.Context, nctx engine.NativeContext) error {
	id := nctx.Args["id"]
	s := store.Open(filePath())
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	n := findNode(cfg, id)
	if n == nil {
		return fmt.Errorf("node %q not found", id)
	}
	fmt.Fprintf(nctx.Stdout, "id:      %s\n", n.ID)
	fmt.Fprintf(nctx.Stdout, "name:    %s\n", n.Name)
	fmt.Fprintf(nctx.Stdout, "enabled: %t\n", n.Enabled)
	fmt.Fprintf(nctx.Stdout, "url:     %s\n", n.URL)
	return nil
}

func runNodeAdd(_ context.Context, nctx engine.NativeContext) error {
	name := nctx.Args["name"]
	link := nctx.Args["url"]
	if name == "" {
		return fmt.Errorf("--name is required")
	}
	if nctx.Args["url-stdin"] == strTrue {
		line, err := readLine(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading url from stdin: %w", err)
		}
		link = line
	}
	if link == "" {
		return fmt.Errorf("--url or --url-stdin is required")
	}

	id, err := store.GenerateID()
	if err != nil {
		return err
	}
	s := store.Open(filePath())
	_, err = s.Update(func(c *store.Config) error {
		c.Nodes = append(c.Nodes, store.Node{ID: id, Name: name, URL: link, Enabled: true})
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(nctx.Stdout, "added node %s\n", id)
	return nil
}

func runNodeRename(_ context.Context, nctx engine.NativeContext) error {
	s := store.Open(filePath())
	_, err := s.Update(func(c *store.Config) error {
		n := findNode(c, nctx.Args["id"])
		if n == nil {
			return fmt.Errorf("node %q not found", nctx.Args["id"])
		}
		n.Name = nctx.Args["name"]
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(nctx.Stdout, "renamed node %s\n", nctx.Args["id"])
	return nil
}

func runNodeEnable(_ context.Context, nctx engine.NativeContext) error {
	return setNodeEnabled(nctx, true)
}

func runNodeDisable(_ context.Context, nctx engine.NativeContext) error {
	return setNodeEnabled(nctx, false)
}

func setNodeEnabled(nctx engine.NativeContext, enabled bool) error {
	id := nctx.Args["id"]
	s := store.Open(filePath())
	_, err := s.Update(func(c *store.Config) error {
		n := findNode(c, id)
		if n == nil {
			return fmt.Errorf("node %q not found", id)
		}
		n.Enabled = enabled
		return nil
	})
	if err != nil {
		return err
	}
	action := "disabled"
	if enabled {
		action = "enabled"
	}
	fmt.Fprintf(nctx.Stdout, "node %s %s\n", id, action)
	return nil
}

func runNodeRemove(_ context.Context, nctx engine.NativeContext) error {
	if nctx.Args["yes"] != strTrue {
		return fmt.Errorf("removal requires --yes")
	}
	id := nctx.Args["id"]
	s := store.Open(filePath())
	found := false
	_, err := s.Update(func(c *store.Config) error {
		out := c.Nodes[:0]
		for _, n := range c.Nodes {
			if n.ID == id {
				found = true
				continue
			}
			out = append(out, n)
		}
		c.Nodes = out
		return nil
	})
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("node %q not found", id)
	}
	fmt.Fprintf(nctx.Stdout, "node %s removed\n", id)
	fmt.Fprintln(nctx.Stdout, "note: Xray on the VPS keeps running; remove the library entry only")
	return nil
}

func findNode(c *store.Config, id string) *store.Node {
	for i := range c.Nodes {
		if c.Nodes[i].ID == id {
			return &c.Nodes[i]
		}
	}
	return nil
}

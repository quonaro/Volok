package cli

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/quonaro/lota/engine"

	"volok/internal/geo"
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
	cyan(nctx.Stdout, "name:    %s\n", n.Name)
	fmt.Fprintf(nctx.Stdout, "enabled: %t\n", n.Enabled)
	fmt.Fprintf(nctx.Stdout, "url:     %s\n", n.URL)
	return nil
}

func runNodeAdd(ctx context.Context, nctx engine.NativeContext) error {
	name := nctx.Args["name"]
	link := nctx.Args["url"]
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
	s := store.Open(filePath())
	if name == "" {
		var used map[int]bool
		if cfg, err := s.Read(); err == nil {
			names := make([]string, 0, len(cfg.Nodes))
			for _, n := range cfg.Nodes {
				names = append(names, n.Name)
			}
			used = geo.SuffixSet(names)
		}
		var err error
		name, err = detectNodeName(ctx, link, used)
		if err != nil {
			return err
		}
	}

	id, err := store.GenerateID()
	if err != nil {
		return err
	}
	_, err = s.Update(func(c *store.Config) error {
		c.Nodes = append(c.Nodes, store.Node{ID: id, Name: name, URL: link, Enabled: true})
		return nil
	})
	if err != nil {
		return err
	}
	green(nctx.Stdout, "added node %s\n", id)
	cyan(nctx.Stdout, "%s\n", name)
	fmt.Fprintln(nctx.Stdout, "note: restart the volok service so the relay picks up this node")
	return nil
}

// detectNodeName derives a display name from the VLESS link by resolving the
// host's country via geo APIs, matching the node.sh installer format:
// "<flag> <Country>#<4 digits>". Falls back to "VPS#<4 digits>" when the
// country cannot be determined. The suffix is unique across used.
func detectNodeName(ctx context.Context, link string, used map[int]bool) (string, error) {
	p, err := vless.Parse(link)
	if err != nil {
		return "", err
	}
	suffix := geo.UniqueSuffix(used)
	ip := p.Host
	if net.ParseIP(ip) == nil {
		resolver := net.DefaultResolver
		ips, err := resolver.LookupIPAddr(ctx, ip)
		if err != nil || len(ips) == 0 {
			return geo.NodeName(geo.Result{}, suffix), nil
		}
		ip = ips[0].IP.String()
	}
	geoCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	result, err := geo.Detect(geoCtx, ip)
	if err != nil {
		return geo.NodeName(geo.Result{}, suffix), nil
	}
	return geo.NodeName(result, suffix), nil
}

func runNodeRename(_ context.Context, nctx engine.NativeContext) error {
	s := store.Open(filePath())
	_, err := s.Update(func(c *store.Config) error {
		n := findNode(c, nctx.Args["id"])
		if n == nil {
			return fmt.Errorf("node %q not found", nctx.Args["id"])
		}
		names := make([]string, 0, len(c.Nodes))
		for _, o := range c.Nodes {
			if o.ID != n.ID {
				names = append(names, o.Name)
			}
		}
		n.Name = geo.EnsureUniqueSuffix(nctx.Args["name"], geo.SuffixSet(names))
		return nil
	})
	if err != nil {
		return err
	}
	green(nctx.Stdout, "renamed node %s\n", nctx.Args["id"])
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
	green(nctx.Stdout, "node %s %s\n", id, action)
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
	green(nctx.Stdout, "node %s removed\n", id)
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

// runNodeNormalize rebuilds every node name that is not already in the
// canonical "<flag> <Country>#<NNNN>" format by resolving the host's country
// via geo-IP. Suffixes are kept unique across all nodes.
func runNodeNormalize(ctx context.Context, nctx engine.NativeContext) error {
	s := store.Open(filePath())
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	used := geo.SuffixSet(nodeNames(cfg.Nodes))
	updates := make(map[string]string, len(cfg.Nodes))
	for _, n := range cfg.Nodes {
		if geo.IsCanonical(n.Name) {
			continue
		}
		name, err := detectNodeName(ctx, n.URL, used)
		if err != nil {
			return fmt.Errorf("node %s: %w", n.ID, err)
		}
		if v, ok := geo.Suffix(name); ok {
			used[v] = true
		}
		updates[n.ID] = name
	}
	if len(updates) == 0 {
		green(nctx.Stdout, "all node names already normalized\n")
		return nil
	}
	_, err = s.Update(func(c *store.Config) error {
		for i := range c.Nodes {
			if name, ok := updates[c.Nodes[i].ID]; ok {
				c.Nodes[i].Name = name
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for id, name := range updates {
		green(nctx.Stdout, "normalized %s\n", id)
		cyan(nctx.Stdout, "  %s\n", name)
	}
	return nil
}

func nodeNames(nodes []store.Node) []string {
	names := make([]string, 0, len(nodes))
	for _, n := range nodes {
		names = append(names, n.Name)
	}
	return names
}

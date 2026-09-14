package cli

import (
	"fmt"
	"os"

	"volok/internal/store"
	"volok/internal/vless"
)

func runNode(a *App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("node requires a subcommand: list|show|add|rename|enable|disable|remove")
	}
	switch args[0] {
	case "list":
		return nodeList(a, args[1:])
	case subShow:
		return nodeShow(a, args[1:])
	case "add":
		return nodeAdd(a, args[1:])
	case "rename":
		return nodeRename(a, args[1:])
	case "enable":
		return nodeEnable(a, args[1:], true)
	case "disable":
		return nodeEnable(a, args[1:], false)
	case "remove":
		return nodeRemove(a, args[1:])
	default:
		return fmt.Errorf("unknown node subcommand %q", args[0])
	}
}

func nodeList(a *App, args []string) error {
	if err := unknownArg(args, "node list"); err != nil {
		return err
	}
	s := store.Open(a.file)
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	if len(cfg.Nodes) == 0 {
		fmt.Fprintln(a.stdout, "no nodes")
		return nil
	}
	for _, n := range cfg.Nodes {
		p, err := vless.Parse(n.URL)
		if err != nil {
			return fmt.Errorf("node %s: %w", n.ID, err)
		}
		state := "disabled"
		if n.Enabled {
			state = "enabled"
		}
		fmt.Fprintf(a.stdout, "%s\t%s\t%s:%d\t%s\n", n.ID, n.Name, p.Host, p.Port, state)
	}
	return nil
}

func nodeShow(a *App, args []string) error {
	if err := requireArgs(args, 1); err != nil {
		return err
	}
	if err := unknownArg(args[1:], "node show"); err != nil {
		return err
	}
	s := store.Open(a.file)
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	n := findNode(cfg, args[0])
	if n == nil {
		return fmt.Errorf("node %q not found", args[0])
	}
	fmt.Fprintf(a.stdout, "id:      %s\n", n.ID)
	fmt.Fprintf(a.stdout, "name:    %s\n", n.Name)
	fmt.Fprintf(a.stdout, "enabled: %t\n", n.Enabled)
	fmt.Fprintf(a.stdout, "url:     %s\n", n.URL)
	return nil
}

func nodeAdd(a *App, args []string) error {
	fs := flagSet("node add", a.stderr)
	name := fs.String("name", "", "display name")
	url := fs.String("url", "", "direct vless:// link")
	urlStdin := fs.Bool("url-stdin", false, "read the link from stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := unknownArg(fs.Args(), "node add"); err != nil {
		return err
	}
	if *name == "" {
		return fmt.Errorf("--name is required")
	}
	link := *url
	if *urlStdin {
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
	s := store.Open(a.file)
	_, err = s.Update(func(c *store.Config) error {
		c.Nodes = append(c.Nodes, store.Node{ID: id, Name: *name, URL: link, Enabled: true})
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "added node %s\n", id)
	return nil
}

func nodeRename(a *App, args []string) error {
	if err := requireArgs(args, 2); err != nil {
		return err
	}
	if err := unknownArg(args[2:], "node rename"); err != nil {
		return err
	}
	s := store.Open(a.file)
	found := false
	_, err := s.Update(func(c *store.Config) error {
		n := findNode(c, args[0])
		if n == nil {
			return fmt.Errorf("node %q not found", args[0])
		}
		n.Name = args[1]
		found = true
		return nil
	})
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("node %q not found", args[0])
	}
	fmt.Fprintf(a.stdout, "renamed node %s\n", args[0])
	return nil
}

func nodeEnable(a *App, args []string, enabled bool) error {
	if err := requireArgs(args, 1); err != nil {
		return err
	}
	if err := unknownArg(args[1:], "node enable/disable"); err != nil {
		return err
	}
	s := store.Open(a.file)
	_, err := s.Update(func(c *store.Config) error {
		n := findNode(c, args[0])
		if n == nil {
			return fmt.Errorf("node %q not found", args[0])
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
	fmt.Fprintf(a.stdout, "node %s %s\n", args[0], action)
	return nil
}

func nodeRemove(a *App, args []string) error {
	fs := flagSet("node remove", a.stderr)
	yes := fs.Bool("yes", false, "confirm removal")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*yes {
		return fmt.Errorf("removal requires --yes")
	}
	rest := fs.Args()
	if len(rest) != 1 {
		return fmt.Errorf("expected exactly one node id")
	}

	s := store.Open(a.file)
	found := false
	_, err := s.Update(func(c *store.Config) error {
		out := c.Nodes[:0]
		for _, n := range c.Nodes {
			if n.ID == rest[0] {
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
		return fmt.Errorf("node %q not found", rest[0])
	}
	fmt.Fprintf(a.stdout, "node %s removed\n", rest[0])
	fmt.Fprintln(a.stdout, "note: Xray on the VPS keeps running; remove the library entry only")
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

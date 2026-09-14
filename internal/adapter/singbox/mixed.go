package singbox

import (
	"fmt"
	"net/netip"

	"outless/internal/domain"
	"outless/internal/utils"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/auth"
)

// buildMixedInbound creates a sing-box mixed (SOCKS5+HTTP) inbound.
func buildMixedInbound(
	index int,
	inbound HubInboundConfig,
	mixedUsers []auth.User,
) (option.Inbound, error) {
	listen := inbound.Listen
	if listen == "" {
		listen = "0.0.0.0"
	}
	listenAddr, err := netip.ParseAddr(listen)
	if err != nil {
		return option.Inbound{}, fmt.Errorf("parsing listen address %q: %w", listen, err)
	}

	port := inbound.Port
	if port == 0 {
		port = 1080
	}

	mixedOpts := option.HTTPMixedInboundOptions{
		ListenOptions: option.ListenOptions{
			Listen:     option.NewListenAddress(listenAddr),
			ListenPort: uint16(port),
		},
		Users: mixedUsers,
	}

	tag := fmt.Sprintf("%s-%d", tagMixedInbound, index)
	return option.Inbound{Type: C.TypeMixed, Tag: tag, MixedOptions: mixedOpts}, nil
}

// buildMixedAuthUsers creates per-node auth users for mixed inbounds.
// Each user maps to the same route rules as VLESS users.
func buildMixedAuthUsers(
	tokens []domain.Token,
	nodes []domain.Node,
) []auth.User {
	users := make([]auth.User, 0)

	for _, token := range tokens {
		if token.UUID == "" {
			continue
		}

		allowed := make(map[string]struct{})
		for _, gid := range token.GroupIDs {
			allowed[gid] = struct{}{}
		}
		if len(allowed) == 0 && token.GroupID != "" {
			allowed[token.GroupID] = struct{}{}
		}
		allGroups := len(allowed) == 0

		for _, node := range nodes {
			if !allGroups {
				nodeAllowed := false
				for _, gid := range node.GroupIDs {
					if _, ok := allowed[gid]; ok {
						nodeAllowed = true
						break
					}
				}
				if !nodeAllowed {
					continue
				}
			}
			name := userName(token.ID, node.ID)
			pass := utils.GeneratePasswordFromTokenNode(token.ID, node.ID)
			users = append(users, auth.User{Username: name, Password: pass})
		}
	}

	return users
}

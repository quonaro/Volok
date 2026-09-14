package service

import (
	"fmt"
	"net/url"

	"outless/internal/domain"
	"outless/internal/utils"
)

const (
	clashTypeSocks5  = "socks5"
	clashTypeHTTP    = "http"
	singboxTypeSocks = "socks"
	singboxTypeHTTP  = "http"
)

// buildMixedV2RayURL builds a socks5:// or http:// URL for a mixed inbound.
func (s *SubscriptionService) buildMixedV2RayURL(node domain.Node, groupNames map[string]string, hub HubConfig, token domain.Token) string {
	remark, ok := s.buildNodeRemark(node, resolveGroupLabel(groupNames, getNodePrimaryGroup(node)), hub, token)
	if !ok {
		return ""
	}

	username := userName(token.ID, node.ID)
	password := utils.GeneratePasswordFromTokenNode(token.ID, node.ID)

	host := s.externalHost
	if host == "" {
		host = "hub.example.com"
	}

	u := url.URL{
		Scheme: "socks5",
		User:   url.UserPassword(username, password),
		Host:   fmt.Sprintf("%s:%d", host, hub.Port),
	}
	u.Fragment = remark
	return u.String()
}

// buildMixedClashMetaProxy builds a Clash Meta socks5 proxy for a mixed inbound.
func (s *SubscriptionService) buildMixedClashMetaProxy(
	node domain.Node,
	groupNames map[string]string,
	hub HubConfig,
	token domain.Token,
) (ClashMetaProxy, string) {
	remark, _ := s.buildNodeRemark(node, resolveGroupLabel(groupNames, getNodePrimaryGroup(node)), hub, token)
	if remark == "" {
		remark = node.ID
	}

	username := userName(token.ID, node.ID)
	password := utils.GeneratePasswordFromTokenNode(token.ID, node.ID)

	return ClashMetaProxy{
		Name:     remark,
		Type:     clashTypeSocks5,
		Server:   s.externalHost,
		Port:     hub.Port,
		Username: username,
		Password: password,
		UDP:      true,
	}, remark
}

// buildMixedSingBoxOutbound builds a Sing-box socks outbound for a mixed inbound.
func (s *SubscriptionService) buildMixedSingBoxOutbound(
	node domain.Node,
	groupNames map[string]string,
	hub HubConfig,
	token domain.Token,
) SingBoxOutbound {
	remark, _ := s.buildNodeRemark(node, resolveGroupLabel(groupNames, getNodePrimaryGroup(node)), hub, token)
	if remark == "" {
		remark = node.ID
	}

	username := userName(token.ID, node.ID)
	password := utils.GeneratePasswordFromTokenNode(token.ID, node.ID)

	return SingBoxOutbound{
		Type:       singboxTypeSocks,
		Tag:        remark,
		Server:     s.externalHost,
		ServerPort: hub.Port,
		Username:   username,
		Password:   password,
	}
}

// buildMixedSurgeProxy builds a Surge socks5 proxy line for a mixed inbound.
func (s *SubscriptionService) buildMixedSurgeProxy(
	node domain.Node,
	groupNames map[string]string,
	hub HubConfig,
	token domain.Token,
) (string, string) {
	remark, ok := s.buildNodeRemark(node, resolveGroupLabel(groupNames, getNodePrimaryGroup(node)), hub, token)
	if !ok {
		return "", ""
	}

	username := userName(token.ID, node.ID)
	password := utils.GeneratePasswordFromTokenNode(token.ID, node.ID)

	line := fmt.Sprintf("%s = socks5, %s, %d, username=%s, password=%s",
		remark, s.externalHost, hub.Port, username, password)

	return line, remark
}

// userName builds a deterministic username for a token+node pair.
func userName(tokenID, nodeID string) string {
	return fmt.Sprintf("t-%s-n-%s", tokenID, nodeID)
}

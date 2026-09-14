package singbox

import (
	"log/slog"

	"outless/internal/domain"
	"outless/shared/vless"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

// buildOutbounds creates one outbound per exit node.
// Self-nodes use a direct outbound; others use VLESS.
func buildOutbounds(nodes []domain.Node, logger *slog.Logger) ([]option.Outbound, error) {
	outbounds := make([]option.Outbound, 0, len(nodes))

	for _, node := range nodes {
		if node.IsSelf {
			outbounds = append(outbounds, option.Outbound{
				Type: C.TypeDirect,
				Tag:  outboundTag(node.ID),
			})
			continue
		}

		parsed, err := vless.ParseURL(node.URL)
		if err != nil {
			if logger != nil {
				logger.Error("failed to parse VLESS URL", slog.String("node", node.ID), slog.String("error", err.Error()))
			}
			continue
		}

		vlessOut := option.VLESSOutboundOptions{
			ServerOptions: option.ServerOptions{Server: parsed.Host, ServerPort: uint16(parsed.Port)},
			UUID:          parsed.UUID,
			Flow:          parsed.Flow,
		}
		if tls := buildOutboundTLS(parsed); tls != nil {
			vlessOut.TLS = tls
		}
		if transport := buildTransport(parsed); transport != nil {
			vlessOut.Transport = transport
		}

		outbounds = append(outbounds, option.Outbound{
			Type:         C.TypeVLESS,
			Tag:          outboundTag(node.ID),
			VLESSOptions: vlessOut,
		})
	}

	return outbounds, nil
}

func buildOutboundTLS(p vless.Parsed) *option.OutboundTLSOptions {
	switch p.Security {
	case "reality":
		tls := &option.OutboundTLSOptions{
			Enabled:    true,
			ServerName: p.SNI,
			Reality:    &option.OutboundRealityOptions{Enabled: true, PublicKey: p.PBK, ShortID: p.SID},
		}
		fp := p.FP
		if fp == "" {
			fp = "chrome"
		}
		tls.UTLS = &option.OutboundUTLSOptions{Enabled: true, Fingerprint: fp}
		return tls
	case "tls":
		tls := &option.OutboundTLSOptions{Enabled: true, ServerName: p.SNI}
		if len(p.ALPN) > 0 {
			tls.ALPN = p.ALPN
		}
		if p.FP != "" {
			tls.UTLS = &option.OutboundUTLSOptions{Enabled: true, Fingerprint: p.FP}
		}
		return tls
	default:
		return nil
	}
}

func buildTransport(p vless.Parsed) *option.V2RayTransportOptions {
	switch p.Network {
	case "ws":
		path := p.Path
		if path == "" {
			path = "/"
		}
		ws := option.V2RayWebsocketOptions{Path: path}
		if p.HostHeader != "" {
			ws.Headers = option.HTTPHeader{"Host": option.Listable[string]{p.HostHeader}}
		}
		return &option.V2RayTransportOptions{Type: C.V2RayTransportTypeWebsocket, WebsocketOptions: ws}
	case "grpc":
		return &option.V2RayTransportOptions{
			Type:        C.V2RayTransportTypeGRPC,
			GRPCOptions: option.V2RayGRPCOptions{ServiceName: p.Service},
		}
	default:
		return nil
	}
}

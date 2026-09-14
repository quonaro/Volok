package http

import (
	"context"
	"log/slog"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"outless/internal/domain"
	"outless/shared/config"
)

type InboundManagementHandler struct {
	inbounds []domain.Inbound
	runtime  RuntimeController
	logger   *slog.Logger
}

func NewInboundManagementHandler(
	inbounds []domain.Inbound,
	runtime RuntimeController,
	logger *slog.Logger,
) *InboundManagementHandler {
	return &InboundManagementHandler{inbounds: inbounds, runtime: runtime, logger: logger}
}

type InboundItem struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	Address      string    `json:"address"`
	Port         int       `json:"port"`
	SNI          string    `json:"sni"`
	Handshake    string    `json:"handshake"`
	PublicKey    string    `json:"public_key"`
	ShortID      string    `json:"short_id"`
	Fingerprint  string    `json:"fingerprint"`
	NameTemplate string    `json:"name_template"`
	Status       string    `json:"status"`
	StatusReason string    `json:"status_reason"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ListInboundsOutput struct {
	Body []InboundItem `json:"inbounds"`
}

type GenerateKeypairOutput struct {
	Body struct {
		PrivateKey string `json:"private_key"`
		PublicKey  string `json:"public_key"`
	}
}

func (h *InboundManagementHandler) Register(api huma.API) {
	huma.Get(api, "/v1/inbounds", h.ListInbounds)
	huma.Get(api, "/v1/inbounds/keypair", h.GenerateKeypair)
}

func (h *InboundManagementHandler) GenerateKeypair(ctx context.Context, _ *struct{}) (*GenerateKeypairOutput, error) {
	priv, pub, err := config.GenerateRealityKeyPair()
	if err != nil {
		h.logger.Error("failed to generate reality key pair", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to generate key pair")
	}
	out := &GenerateKeypairOutput{}
	out.Body.PrivateKey = priv
	out.Body.PublicKey = pub
	return out, nil
}

func (h *InboundManagementHandler) ListInbounds(ctx context.Context, _ *struct{}) (*ListInboundsOutput, error) {
	items := make([]InboundItem, 0, len(h.inbounds))
	for _, inbound := range h.inbounds {
		status, reason := h.runtime.InboundStatus(inbound.ID)
		items = append(items, toInboundItem(inbound, status, reason))
	}

	out := &ListInboundsOutput{}
	out.Body = items
	return out, nil
}

func toInboundItem(inbound domain.Inbound, status, reason string) InboundItem {
	return InboundItem{
		ID:           inbound.ID,
		Name:         inbound.Name,
		Type:         inbound.Type,
		Address:      inbound.Address,
		Port:         inbound.Port,
		SNI:          inbound.SNI,
		Handshake:    inbound.Handshake,
		PublicKey:    inbound.PublicKey,
		ShortID:      inbound.ShortID,
		Fingerprint:  inbound.Fingerprint,
		NameTemplate: inbound.NameTemplate,
		Status:       status,
		StatusReason: reason,
		CreatedAt:    inbound.CreatedAt,
		UpdatedAt:    inbound.UpdatedAt,
	}
}

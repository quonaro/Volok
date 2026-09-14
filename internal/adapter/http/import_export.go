package http

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"

	"outless/internal/domain"
)

// ImportExportHandler handles configuration export and import.
type ImportExportHandler struct {
	nodeRepo         domain.NodeRepository
	tokenRepo        domain.TokenRepository
	groupRepo        domain.GroupRepository
	publicSourceRepo domain.PublicSourceRepository
	logger           *slog.Logger
}

// NewImportExportHandler constructs an import/export handler.
func NewImportExportHandler(
	nodeRepo domain.NodeRepository,
	tokenRepo domain.TokenRepository,
	groupRepo domain.GroupRepository,
	publicSourceRepo domain.PublicSourceRepository,
	logger *slog.Logger,
) *ImportExportHandler {
	return &ImportExportHandler{
		nodeRepo:         nodeRepo,
		tokenRepo:        tokenRepo,
		groupRepo:        groupRepo,
		publicSourceRepo: publicSourceRepo,
		logger:           logger,
	}
}

// exportNode is a serializable node representation.
type exportNode struct {
	ID          string   `json:"id"`
	URL         string   `json:"url"`
	GroupIDs    []string `json:"group_ids"`
	Country     string   `json:"country"`
	CountryCode string   `json:"country_code"`
	CountryName string   `json:"country_name"`
	CountryFlag string   `json:"country_flag"`
	IsSelf      bool     `json:"is_self"`
	ExpiresAt   string   `json:"expires_at,omitempty"`
}

// exportGroup is a serializable group representation.
type exportGroup struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	InboundID     string `json:"inbound_id"`
	RandomEnabled bool   `json:"random_enabled"`
	RandomLimit   *int   `json:"random_limit,omitempty"`
}

// exportPublicSource is a serializable public source representation.
type exportPublicSource struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	GroupID string `json:"group_id"`
}

// exportToken is a serializable token representation without secrets.
type exportToken struct {
	Owner       string   `json:"owner"`
	GroupIDs    []string `json:"group_ids"`
	IsActive    bool     `json:"is_active"`
	QuotaBytes  *int64   `json:"quota_bytes,omitempty"`
	QuotaPeriod string   `json:"quota_period"`
	ExpiresAt   string   `json:"expires_at"`
}

// ExportOutput holds the full configuration export.
type ExportOutput struct {
	Body struct {
		Nodes         []exportNode         `json:"nodes"`
		Groups        []exportGroup        `json:"groups"`
		PublicSources []exportPublicSource `json:"public_sources"`
		Tokens        []exportToken        `json:"tokens"`
	}
}

// ImportInput accepts a configuration to import. Groups and nodes are required;
// public_sources and tokens are optional.
type ImportInput struct {
	Body struct {
		Nodes         []exportNode         `json:"nodes"`
		Groups        []exportGroup        `json:"groups"`
		PublicSources []exportPublicSource `json:"public_sources,omitempty"`
		Tokens        []exportToken        `json:"tokens,omitempty"`
	}
}

// Register wires import/export endpoints into Huma API.
func (h *ImportExportHandler) Register(api huma.API) {
	huma.Get(api, "/v1/export", h.Export)
	huma.Post(api, "/v1/import", h.Import)
}

// Export dumps the current database configuration.
func (h *ImportExportHandler) Export(ctx context.Context, _ *struct{}) (*ExportOutput, error) {
	out := &ExportOutput{}
	var err error

	out.Body.Nodes, err = h.exportNodes(ctx)
	if err != nil {
		return nil, err
	}

	out.Body.Groups, err = h.exportGroups(ctx)
	if err != nil {
		return nil, err
	}

	out.Body.PublicSources, err = h.exportPublicSources(ctx)
	if err != nil {
		return nil, err
	}

	out.Body.Tokens, err = h.exportTokens(ctx)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// Import loads a full configuration into the database.
func (h *ImportExportHandler) Import(ctx context.Context, input *ImportInput) (*struct{}, error) {
	const maxItems = 10000
	if len(input.Body.Nodes) > maxItems || len(input.Body.Groups) > maxItems ||
		len(input.Body.PublicSources) > maxItems || len(input.Body.Tokens) > maxItems {
		return nil, huma.Error400BadRequest(fmt.Sprintf("too many items: max %d per category", maxItems))
	}

	h.importGroups(ctx, input.Body.Groups)
	h.importNodes(ctx, input.Body.Nodes)
	h.importPublicSources(ctx, input.Body.PublicSources)
	h.importTokens(ctx, input.Body.Tokens)

	h.logger.Info("configuration imported",
		slog.Int("groups", len(input.Body.Groups)),
		slog.Int("nodes", len(input.Body.Nodes)),
		slog.Int("public_sources", len(input.Body.PublicSources)),
		slog.Int("tokens", len(input.Body.Tokens)),
	)
	return nil, nil
}

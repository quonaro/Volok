package http

import (
	"context"
	"log/slog"
	"time"

	"outless/internal/country"
	"outless/internal/domain"
)

func (h *ImportExportHandler) importGroups(ctx context.Context, groups []exportGroup) {
	for _, g := range groups {
		if g.ID == "" || g.Name == "" {
			h.logger.Warn("import group skipped: missing id or name", slog.String("id", g.ID))
			continue
		}
		if len(g.Name) > 256 {
			h.logger.Warn("import group skipped: name too long", slog.String("id", g.ID))
			continue
		}
		if err := h.groupRepo.Create(ctx, domain.Group{
			ID:            g.ID,
			Name:          g.Name,
			InboundID:     g.InboundID,
			RandomEnabled: g.RandomEnabled,
			RandomLimit:   g.RandomLimit,
			CreatedAt:     time.Now().UTC(),
		}); err != nil {
			h.logger.Warn("import group skipped", slog.String("id", g.ID), slog.String("error", err.Error()))
		}
	}
}

func (h *ImportExportHandler) importNodes(ctx context.Context, nodes []exportNode) {
	for _, n := range nodes {
		if n.ID == "" || n.URL == "" {
			h.logger.Warn("import node skipped: missing id or url")
			continue
		}
		if len(n.URL) > 4096 {
			h.logger.Warn("import node skipped: url too long", slog.String("id", n.ID))
			continue
		}
		if len(n.GroupIDs) > 100 {
			h.logger.Warn("import node skipped: too many group_ids", slog.String("id", n.ID))
			continue
		}

		node := domain.Node{
			ID:       n.ID,
			URL:      n.URL,
			GroupIDs: n.GroupIDs,
			Country:  n.Country,
			IsSelf:   n.IsSelf,
		}

		code := domain.NormalizeCountryCode(n.CountryCode)
		if code == "" {
			code = domain.NormalizeCountryCode(n.Country)
		}
		if code != "" {
			flag := n.CountryFlag
			if flag == "" {
				flag = country.FlagEmoji(code)
			}
			node.CountryInfo = &domain.CountryInfo{
				CountryCode: code,
				CountryName: n.CountryName,
				Flag:        flag,
			}
		}

		if n.ExpiresAt != "" {
			expiresAt, err := time.Parse(time.RFC3339, n.ExpiresAt)
			if err != nil {
				h.logger.Warn("import node skipped", slog.String("id", n.ID), slog.String("error", err.Error()))
				continue
			}
			node.ExpiresAt = &expiresAt
		}
		if err := h.nodeRepo.Upsert(ctx, node); err != nil {
			h.logger.Warn("import node skipped", slog.String("id", n.ID), slog.String("error", err.Error()))
		}
	}
}

func (h *ImportExportHandler) importPublicSources(ctx context.Context, sources []exportPublicSource) {
	for _, ps := range sources {
		if ps.ID == "" || ps.URL == "" {
			h.logger.Warn("import public source skipped: missing id or url")
			continue
		}
		if len(ps.URL) > 4096 {
			h.logger.Warn("import public source skipped: url too long", slog.String("id", ps.ID))
			continue
		}
		if err := h.publicSourceRepo.Create(ctx, domain.PublicSource{
			ID:        ps.ID,
			URL:       ps.URL,
			GroupID:   ps.GroupID,
			CreatedAt: time.Now().UTC(),
		}); err != nil {
			h.logger.Warn("import public source skipped", slog.String("id", ps.ID), slog.String("error", err.Error()))
		}
	}
}

func (h *ImportExportHandler) importTokens(ctx context.Context, tokens []exportToken) {
	for _, t := range tokens {
		if t.Owner == "" {
			h.logger.Warn("import token skipped: missing owner")
			continue
		}
		if len(t.Owner) > 256 {
			h.logger.Warn("import token skipped: owner too long")
			continue
		}
		expiresAt, _ := time.Parse(time.RFC3339, t.ExpiresAt)
		if expiresAt.IsZero() {
			expiresAt = time.Now().UTC().Add(30 * 24 * time.Hour)
		}
		if _, _, err := h.tokenRepo.IssueToken(
			ctx, t.Owner, t.GroupIDs, expiresAt, t.QuotaBytes, t.QuotaPeriod,
		); err != nil {
			h.logger.Warn("import token skipped", slog.String("owner", t.Owner), slog.String("error", err.Error()))
		}
	}
}

package http

import (
	"context"
	"errors"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"

	"outless/internal/domain"
)

type UpdateNodeInput struct {
	ID   string `path:"id" required:"true"`
	Body struct {
		URL       string   `json:"url,omitempty"`
		GroupIDs  []string `json:"group_ids,omitempty"`
		ExpiresAt *string  `json:"expires_at,omitempty"`
	}
}

func (h *NodeManagementHandler) UpdateNode(ctx context.Context, input *UpdateNodeInput) (*struct{}, error) {
	if input.Body.URL == "" && len(input.Body.GroupIDs) == 0 && input.Body.ExpiresAt == nil {
		return nil, huma.Error400BadRequest("at least one field (url, group_ids or expires_at) is required")
	}

	existingNode, err := h.nodeRepo.FindByID(ctx, input.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNodeNotFound) {
			return nil, huma.Error404NotFound("node not found")
		}
		h.logger.Error("failed to find node for update", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to find node")
	}

	updates := domain.Node{
		ID:        input.ID,
		URL:       existingNode.URL,
		GroupIDs:  existingNode.GroupIDs,
		ExpiresAt: existingNode.ExpiresAt,
	}

	if input.Body.URL != "" {
		updates.URL = input.Body.URL
	}

	if input.Body.ExpiresAt != nil {
		expiresAt, err := parseOptionalExpiresAt(input.Body.ExpiresAt)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid expires_at")
		}
		updates.ExpiresAt = expiresAt
	}

	if len(input.Body.GroupIDs) > 0 {
		updates.GroupIDs = input.Body.GroupIDs
	}

	if err := h.nodeRepo.Update(ctx, updates); err != nil {
		h.logger.Error("failed to update node", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to update node")
	}

	if input.Body.URL != "" && input.Body.URL != existingNode.URL {
		if err := h.nodeRepo.ResetCountryInfo(ctx, input.ID); err != nil {
			h.logger.Warn("failed to reset country info after url change", slog.String("id", input.ID), slog.String("error", err.Error()))
		}
	}

	if err := h.runtime.ForceSync(); err != nil {
		h.logger.Warn("failed to sync after node update", slog.String("id", input.ID), slog.String("error", err.Error()))
	}

	return nil, nil
}

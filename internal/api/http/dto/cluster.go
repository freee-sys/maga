package dto

import "netcluster/internal/domain"

type ClusterResponse struct {
	ID          string `json:"id"`
	LookupKey   string `json:"lookup_key"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	MemberCount int    `json:"member_count"`
}

func ClusterFromDomain(c domain.ClusterWithCount) ClusterResponse {
	return ClusterResponse{
		ID:          c.ID.String(),
		LookupKey:   c.LookupKey,
		DisplayName: c.DisplayName,
		Description: c.Description,
		MemberCount: c.MemberCount,
	}
}

type ClusterRenameRequest struct {
	DisplayName string  `json:"display_name"`
	Description *string `json:"description,omitempty"`
}

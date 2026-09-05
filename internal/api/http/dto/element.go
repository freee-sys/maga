package dto

import (
	"time"

	"netcluster/internal/domain"
)

type ElementResponse struct {
	ID               string    `json:"id"`
	IPAddress        string    `json:"ip_address"`
	SysName          string    `json:"sys_name,omitempty"`
	AssignmentStatus string    `json:"assignment_status"`
	ClusterID        string    `json:"cluster_id,omitempty"`
	LastRuleID       string    `json:"last_rule_id,omitempty"`
	FirstSeenAt      time.Time `json:"first_seen_at,omitzero"`
	LastSeenAt       time.Time `json:"last_seen_at,omitzero"`
}

func ElementFromDomain(e domain.NetworkElement) ElementResponse {
	resp := ElementResponse{
		ID:               e.ID.String(),
		IPAddress:        e.IPAddress,
		SysName:          e.SysName,
		AssignmentStatus: e.AssignmentStatus,
		FirstSeenAt:      e.FirstSeenAt,
		LastSeenAt:       e.LastSeenAt,
	}
	if e.ClusterID != nil {
		resp.ClusterID = e.ClusterID.String()
	}
	if e.LastRuleID != nil {
		resp.LastRuleID = e.LastRuleID.String()
	}
	return resp
}

type AssignmentHistoryResponse struct {
	ID                   string            `json:"id"`
	ClusterID            string            `json:"cluster_id,omitempty"`
	RuleID               string            `json:"rule_id,omitempty"`
	MatchedPattern       string            `json:"matched_pattern,omitempty"`
	RawCaptures          map[string]string `json:"raw_captures,omitempty"`
	TransformedCaptures  map[string]string `json:"transformed_captures,omitempty"`
	RenderedClusterKey   string            `json:"rendered_cluster_key,omitempty"`
	TriggerSource        string            `json:"trigger_source"`
	Warnings             string            `json:"warnings,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`
}

func AssignmentHistoryFromDomain(h domain.AssignmentHistory) AssignmentHistoryResponse {
	resp := AssignmentHistoryResponse{
		ID:                  h.ID.String(),
		MatchedPattern:      h.MatchedPattern,
		RawCaptures:         h.RawCaptures,
		TransformedCaptures: h.TransformedCaptures,
		RenderedClusterKey:  h.RenderedClusterKey,
		TriggerSource:       h.TriggerSource,
		Warnings:            h.Warnings,
		CreatedAt:           h.CreatedAt,
	}
	if h.ClusterID != nil {
		resp.ClusterID = h.ClusterID.String()
	}
	if h.RuleID != nil {
		resp.RuleID = h.RuleID.String()
	}
	return resp
}

type BulkRedistributeRequest struct {
	ElementIDs []string `json:"element_ids,omitempty"`
}

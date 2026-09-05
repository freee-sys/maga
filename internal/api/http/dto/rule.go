// Package dto holds request/response shapes for the HTTP API, kept
// separate from internal/domain so the wire format can evolve
// independently of the internal model.
package dto

import (
	"github.com/google/uuid"

	"netcluster/internal/dsl"
	"netcluster/internal/domain"
)

type RuleRequest struct {
	Name                string                  `json:"name"`
	Description         string                  `json:"description"`
	Pattern             string                  `json:"pattern"`
	CaptureTransforms   map[string]dsl.Pipeline `json:"capture_transforms"`
	ClusterKeyTemplate  string                  `json:"cluster_key_template"`
	Priority            int                     `json:"priority"`
	IsEnabled           bool                    `json:"is_enabled"`
}

type RuleResponse struct {
	ID                 string                  `json:"id"`
	Name               string                  `json:"name"`
	Description        string                  `json:"description"`
	Pattern            string                  `json:"pattern"`
	CaptureTransforms  map[string]dsl.Pipeline `json:"capture_transforms"`
	ClusterKeyTemplate string                  `json:"cluster_key_template"`
	Priority           int                     `json:"priority"`
	IsEnabled          bool                    `json:"is_enabled"`
}

func (req RuleRequest) ToDomain() domain.Rule {
	return domain.Rule{
		Name:               req.Name,
		Description:        req.Description,
		Pattern:            req.Pattern,
		CaptureTransforms:  req.CaptureTransforms,
		ClusterKeyTemplate: req.ClusterKeyTemplate,
		Priority:           req.Priority,
		IsEnabled:          req.IsEnabled,
	}
}

func RuleFromDomain(r domain.Rule) RuleResponse {
	return RuleResponse{
		ID:                 r.ID.String(),
		Name:               r.Name,
		Description:        r.Description,
		Pattern:            r.Pattern,
		CaptureTransforms:  r.CaptureTransforms,
		ClusterKeyTemplate: r.ClusterKeyTemplate,
		Priority:           r.Priority,
		IsEnabled:          r.IsEnabled,
	}
}

type ReorderRequest struct {
	RuleIDs []uuid.UUID `json:"rule_ids"`
}

type RuleTestRequest struct {
	Hostname            string                  `json:"hostname"`
	Pattern             string                  `json:"pattern"`
	CaptureTransforms   map[string]dsl.Pipeline `json:"capture_transforms"`
	ClusterKeyTemplate  string                  `json:"cluster_key_template"`
}

type RuleTestResponse struct {
	Matched               bool              `json:"matched"`
	RawCaptures           map[string]string `json:"raw_captures"`
	TransformedCaptures   map[string]string `json:"transformed_captures"`
	RenderedClusterKey    string            `json:"rendered_cluster_key"`
	WouldCreateNewCluster bool              `json:"would_create_new_cluster"`
	Warnings              []string          `json:"warnings,omitempty"`
}

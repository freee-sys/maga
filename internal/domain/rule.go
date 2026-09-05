package domain

import (
	"github.com/google/uuid"

	"netcluster/internal/dsl"
)

// Rule matches a device hostname against a regex and, on match, derives a
// cluster key from the (optionally transformed) named capture groups.
type Rule struct {
	ID                 uuid.UUID
	Name               string
	Description        string
	Pattern            string
	CaptureTransforms  map[string]dsl.Pipeline
	ClusterKeyTemplate string
	Priority           int
	IsEnabled          bool
}

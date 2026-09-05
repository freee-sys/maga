package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	TriggerSourceDiscovery                = "discovery"
	TriggerSourceManualRedistributeSingle = "manual_redistribute_single"
	TriggerSourceManualRedistributeBulk   = "manual_redistribute_bulk"
)

// AssignmentHistory is one audit record of an assignment attempt for an
// element, successful or not, so the UI can show why/when a device landed
// (or failed to land) in a cluster.
type AssignmentHistory struct {
	ID                   uuid.UUID
	ElementID            uuid.UUID
	ClusterID            *uuid.UUID
	RuleID               *uuid.UUID
	MatchedPattern       string
	RawCaptures          map[string]string
	TransformedCaptures  map[string]string
	RenderedClusterKey   string
	TriggerSource        string
	Warnings             string
	CreatedAt            time.Time
}

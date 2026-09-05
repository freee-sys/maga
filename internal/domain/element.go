package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	AssignmentStatusUnassigned = "unassigned"
	AssignmentStatusAssigned   = "assigned"
)

// ElementFilter narrows an elements listing. A zero value matches
// everything and returns the first page at the default size. Defined
// here (not in the API layer) so storage/postgres can consume it without
// depending on internal/api/http.
type ElementFilter struct {
	ClusterID        *uuid.UUID
	AssignmentStatus string
	Query            string
	Limit            int
	Offset           int
}

// NetworkElement is a device found (or previously found) on the network.
type NetworkElement struct {
	ID               uuid.UUID
	IPAddress        string
	SysName          string
	SysDescr         string
	SysObjectID      string
	SysUptimeTicks   int64
	Attributes       map[string]any
	ClusterID        *uuid.UUID
	AssignmentStatus string
	LastRuleID       *uuid.UUID
	FirstSeenAt      time.Time
	LastSeenAt       time.Time
}

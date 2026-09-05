package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	DiscoveryJobStatusPending   = "pending"
	DiscoveryJobStatusRunning   = "running"
	DiscoveryJobStatusCompleted = "completed"
	DiscoveryJobStatusFailed    = "failed"
	DiscoveryJobStatusCancelled = "cancelled"
)

const (
	SNMPVersionV1  = "v1"
	SNMPVersionV2c = "v2c"
)

// DiscoveryJob is one async SNMP scan run against an expanded set of
// target IPs. Individual host failures never fail the job as a whole —
// Status/ErrorMessage track infra-level problems only (e.g. failing to
// expand targets or open a socket).
type DiscoveryJob struct {
	ID            uuid.UUID
	JobType       string
	InputSpec     string
	TargetIPs     []string
	SNMPVersion   string
	SNMPCommunity string
	Status        string
	TotalTargets  int
	StartedAt     *time.Time
	CompletedAt   *time.Time
	ErrorMessage  string
	CreatedAt     time.Time
}

const (
	DiscoveryResultStatusSuccess    = "success"
	DiscoveryResultStatusNoResponse = "no_response"
	DiscoveryResultStatusSNMPError  = "snmp_error"
)

// DiscoveryJobResult is the per-IP outcome of one scan.
type DiscoveryJobResult struct {
	ID           uuid.UUID
	JobID        uuid.UUID
	IPAddress    string
	Status       string
	SysName      string
	SysDescr     string
	SysObjectID  string
	ErrorMessage string
	ElementID    *uuid.UUID
	RespondedAt  *time.Time
}

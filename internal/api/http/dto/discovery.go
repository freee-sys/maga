package dto

import (
	"time"

	"netcluster/internal/domain"
)

type DiscoveryJobRequest struct {
	Targets       string `json:"targets"`
	SNMPVersion   string `json:"snmp_version"`
	SNMPCommunity string `json:"snmp_community"`
}

type DiscoveryJobResponse struct {
	ID            string     `json:"id"`
	InputSpec     string     `json:"input_spec"`
	SNMPVersion   string     `json:"snmp_version"`
	Status        string     `json:"status"`
	TotalTargets  int        `json:"total_targets"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	ErrorMessage  string     `json:"error_message,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

func DiscoveryJobFromDomain(j domain.DiscoveryJob) DiscoveryJobResponse {
	return DiscoveryJobResponse{
		ID:           j.ID.String(),
		InputSpec:    j.InputSpec,
		SNMPVersion:  j.SNMPVersion,
		Status:       j.Status,
		TotalTargets: j.TotalTargets,
		StartedAt:    j.StartedAt,
		CompletedAt:  j.CompletedAt,
		ErrorMessage: j.ErrorMessage,
		CreatedAt:    j.CreatedAt,
	}
}

type DiscoveryJobResultResponse struct {
	IPAddress    string     `json:"ip_address"`
	Status       string     `json:"status"`
	SysName      string     `json:"sys_name,omitempty"`
	SysDescr     string     `json:"sys_descr,omitempty"`
	SysObjectID  string     `json:"sys_object_id,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
	ElementID    string     `json:"element_id,omitempty"`
	RespondedAt  *time.Time `json:"responded_at,omitempty"`
}

func DiscoveryJobResultFromDomain(r domain.DiscoveryJobResult) DiscoveryJobResultResponse {
	resp := DiscoveryJobResultResponse{
		IPAddress:    r.IPAddress,
		Status:       r.Status,
		SysName:      r.SysName,
		SysDescr:     r.SysDescr,
		SysObjectID:  r.SysObjectID,
		ErrorMessage: r.ErrorMessage,
		RespondedAt:  r.RespondedAt,
	}
	if r.ElementID != nil {
		resp.ElementID = r.ElementID.String()
	}
	return resp
}

package domain

import "github.com/google/uuid"

// Cluster is a dynamically created target group. LookupKey is the exact
// string a rule's cluster-key template renders to, and is what the rule
// engine matches or creates against; DisplayName is freely renameable and
// purely cosmetic, so renaming a cluster never breaks future auto-matching.
type Cluster struct {
	ID          uuid.UUID
	LookupKey   string
	DisplayName string
	Description string
}

// ClusterWithCount is a Cluster plus how many elements currently belong
// to it, for list views.
type ClusterWithCount struct {
	Cluster
	MemberCount int
}

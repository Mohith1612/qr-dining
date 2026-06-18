package authz

import "github.com/google/uuid"

type Scope struct {
	OrganizationID int64
	BranchID       int64
	SessionID      uuid.UUID
}

func (s Scope) SameBranch(other Scope) bool {
	return s.BranchID != 0 && s.BranchID == other.BranchID
}

func (s Scope) SameOrganization(other Scope) bool {
	return s.OrganizationID != 0 && s.OrganizationID == other.OrganizationID
}

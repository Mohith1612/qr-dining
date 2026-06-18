package authz

import (
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
)

type ActorType string

const (
	ActorTypeStaff ActorType = "staff"
)

type Actor struct {
	Type    ActorType
	ID      int64
	Role    sqlc.StaffRole
	Scope   Scope
	Session string
}

func StaffActor(staffID int64, role sqlc.StaffRole, branchID int64, organizationID int64, session string) Actor {
	return Actor{
		Type: ActorTypeStaff,
		ID:   staffID,
		Role: role,
		Scope: Scope{
			OrganizationID: organizationID,
			BranchID:       branchID,
		},
		Session: session,
	}
}

package authz

import (
	"strconv"

	"github.com/google/uuid"
)

func int64String(id int64) string {
	return strconv.FormatInt(id, 10)
}

func OrderResource(id uuid.UUID, branchID int64, sessionID uuid.UUID, organizationID int64) Resource {
	return Resource{Type: ResourceTypeOrder, ID: id.String(), Scope: Scope{OrganizationID: organizationID, BranchID: branchID, SessionID: sessionID}}
}

func AssistanceResource(id int64, branchID int64, sessionID uuid.UUID, organizationID int64) Resource {
	return Resource{Type: ResourceTypeAssistance, ID: int64String(id), Scope: Scope{OrganizationID: organizationID, BranchID: branchID, SessionID: sessionID}}
}

func MenuItemResource(id int64, branchID int64, organizationID int64) Resource {
	return Resource{Type: ResourceTypeMenuItem, ID: int64String(id), Scope: Scope{OrganizationID: organizationID, BranchID: branchID}}
}

func MenuCategoryResource(id int64, branchID int64, organizationID int64) Resource {
	return Resource{Type: ResourceTypeMenuCategory, ID: int64String(id), Scope: Scope{OrganizationID: organizationID, BranchID: branchID}}
}

func MenuModifierResource(id int64, branchID int64, organizationID int64) Resource {
	return Resource{Type: ResourceTypeMenuModifier, ID: int64String(id), Scope: Scope{OrganizationID: organizationID, BranchID: branchID}}
}

func PromoResource(id int64, branchID int64, organizationID int64) Resource {
	return Resource{Type: ResourceTypePromo, ID: int64String(id), Scope: Scope{OrganizationID: organizationID, BranchID: branchID}}
}

func StaffResource(id int64, branchID int64, organizationID int64) Resource {
	return Resource{Type: ResourceTypeStaff, ID: int64String(id), Scope: Scope{OrganizationID: organizationID, BranchID: branchID}}
}

func CustomerResource(id int64, restaurantID int64) Resource {
	return Resource{Type: ResourceTypeCustomer, ID: int64String(id), Scope: Scope{OrganizationID: restaurantID}}
}

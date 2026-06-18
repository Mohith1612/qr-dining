package authz

type ResourceType string

const (
	ResourceTypeOrganization ResourceType = "organization"
	ResourceTypeBranch       ResourceType = "branch"
	ResourceTypeStaff        ResourceType = "staff"
	ResourceTypeMenuItem     ResourceType = "menu_item"
	ResourceTypeMenuCategory ResourceType = "menu_category"
	ResourceTypeMenuModifier ResourceType = "menu_modifier"
	ResourceTypeOrder        ResourceType = "order"
	ResourceTypeAssistance   ResourceType = "assistance"
	ResourceTypePayment      ResourceType = "payment"
	ResourceTypePromo        ResourceType = "promo"
	ResourceTypeAudit        ResourceType = "audit"
	ResourceTypeCustomer     ResourceType = "customer"
)

type Resource struct {
	Type  ResourceType
	ID    string
	Scope Scope
}

func BranchResource(id int64, organizationID int64) Resource {
	return Resource{Type: ResourceTypeBranch, ID: int64String(id), Scope: Scope{OrganizationID: organizationID, BranchID: id}}
}

func OrganizationResource(id int64) Resource {
	return Resource{Type: ResourceTypeOrganization, ID: int64String(id), Scope: Scope{OrganizationID: id}}
}

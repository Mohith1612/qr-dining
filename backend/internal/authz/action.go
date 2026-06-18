package authz

type Action string

const (
	ActionOrganizationRead        Action = "organization.read"
	ActionOrganizationUpdate      Action = "organization.update"
	ActionBranchRead              Action = "branch.read"
	ActionBranchUpdateSettings    Action = "branch.update_settings"
	ActionStaffCreate             Action = "staff.create"
	ActionStaffUpdateRole         Action = "staff.update_role"
	ActionStaffDeactivate         Action = "staff.deactivate"
	ActionMenuItemUpdate          Action = "menu.item.update"
	ActionMenuItemToggleAvailable Action = "menu.item.toggle_availability"
	ActionOrderStatusUpdate       Action = "order.status.update"
	ActionAssistanceAck           Action = "assistance.ack"
	ActionAssistanceResolve       Action = "assistance.resolve"
	ActionPaymentSettleStaff      Action = "payment.settle.staff"
	ActionPromoCreate             Action = "promo.create"
	ActionPromoDeactivate         Action = "promo.deactivate"
	ActionAuditReadBranch         Action = "audit.read.branch"

	ActionMenuCategoryUpdate Action = "menu.category.update"
	ActionMenuModifierUpdate Action = "menu.modifier.update"
	ActionStaffPinUpdate     Action = "staff.pin.update"
	ActionCustomerHistory    Action = "customer.history.read"
	ActionCustomerDelete     Action = "customer.delete"
)

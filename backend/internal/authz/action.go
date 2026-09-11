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
	ActionOrderMarkServed         Action = "order.status.serve"
	ActionAssistanceAck           Action = "assistance.ack"
	ActionAssistanceResolve       Action = "assistance.resolve"
	ActionPaymentSettleStaff      Action = "payment.settle.staff"
	ActionPaymentCancelStaff      Action = "payment.cancel.staff"
	ActionSessionForceClose       Action = "session.force_close"
	ActionPromoCreate             Action = "promo.create"
	ActionPromoDeactivate         Action = "promo.deactivate"
	ActionAuditReadBranch         Action = "audit.read.branch"

	ActionMenuCategoryUpdate Action = "menu.category.update"
	ActionMenuModifierUpdate Action = "menu.modifier.update"
	ActionStaffPinUpdate     Action = "staff.pin.update"
	ActionStaffPinReset      Action = "staff.pin.reset"
	ActionStaffListRead      Action = "staff.list.read"
	ActionCustomerHistory    Action = "customer.history.read"
	ActionCustomerDelete     Action = "customer.delete"
)

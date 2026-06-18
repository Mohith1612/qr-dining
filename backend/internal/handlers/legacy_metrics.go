package handlers

import "github.com/Mohith1612/qr-dining/internal/observability"

const (
	legacyMechanismHeaderParticipantID       = "x_participant_id"
	legacyMechanismBodyParticipantID         = "body_participant_id"
	legacyMechanismBodyPlacedByParticipantID = "body_placed_by_participant_id"
	legacyMechanismBranchPIN                 = "branch_id_pin"
	legacyMechanismWSQueryParticipantID      = "ws_query_participant_id"

	legacyEndpointCart       = "guest_cart"
	legacyEndpointSession    = "guest_session"
	legacyEndpointOrder      = "guest_order"
	legacyEndpointAssistance = "guest_assistance"
	legacyEndpointStaffAuth  = "staff_auth"
	legacyEndpointWebSocket  = "websocket"
)

func recordLegacyIdentityUsage(metrics *observability.Metrics, mechanism, endpointClass string) {
	if metrics == nil || metrics.LegacyIdentityUsageTotal == nil {
		return
	}
	metrics.LegacyIdentityUsageTotal.WithLabelValues(mechanism, endpointClass).Inc()
}

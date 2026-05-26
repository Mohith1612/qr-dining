package services

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/google/uuid"
)

type CustomerService struct {
	repos *repository.Repos
}

func NewCustomerService(repos *repository.Repos) *CustomerService {
	return &CustomerService{repos: repos}
}

type LinkCustomerRequest struct {
	SessionID   uuid.UUID
	Phone       string
	DisplayName string
}

var nonDigit = regexp.MustCompile(`\D`)

func normalizePhone(raw string) (string, error) {
	if strings.HasPrefix(raw, "+") {
		digits := nonDigit.ReplaceAllString(raw, "")
		if len(digits) >= 10 && len(digits) <= 15 {
			return "+" + digits, nil
		}
		return "", domain.ErrInvalidPhone
	}
	digits := nonDigit.ReplaceAllString(raw, "")
	if len(digits) == 10 && digits[0] >= '6' {
		return "+91" + digits, nil
	}
	return "", domain.ErrInvalidPhone
}

func (s *CustomerService) LinkCustomer(ctx context.Context, req LinkCustomerRequest) error {
	phone, err := normalizePhone(req.Phone)
	if err != nil {
		return err
	}

	session, err := s.repos.GetSessionByID(ctx, req.SessionID)
	if err != nil {
		return err
	}

	restaurant, err := s.repos.GetRestaurantByBranchID(ctx, session.BranchID)
	if err != nil {
		return err
	}

	var settings struct {
		CustomerMemoryEnabled bool `json:"customer_memory_enabled"`
	}
	if err := json.Unmarshal(restaurant.SettingsJson, &settings); err == nil {
		if !settings.CustomerMemoryEnabled {
			return domain.ErrFeatureDisabled
		}
	} else {
		return domain.ErrFeatureDisabled
	}

	customer, err := s.repos.UpsertCustomer(ctx, restaurant.ID, phone, req.DisplayName)
	if err != nil {
		return err
	}

	return s.repos.LinkSessionToCustomer(ctx, req.SessionID, customer.ID)
}

func (s *CustomerService) SearchCustomers(ctx context.Context, restaurantID int64, prefix string) ([]CustomerListItem, error) {
	customers, err := s.repos.SearchCustomersByPhone(ctx, restaurantID, prefix)
	if err != nil {
		return nil, err
	}
	result := make([]CustomerListItem, len(customers))
	for i, c := range customers {
		result[i] = CustomerListItem{
			ID:          c.ID,
			PhoneMasked: maskPhone(c.PhoneE164),
			DisplayName: c.DisplayName,
			VisitCount:  int(c.VisitCount),
			LastSeenAt:  c.LastSeenAt.String(),
			OptedIn:     c.OptedIn,
		}
	}
	return result, nil
}

func (s *CustomerService) GetCustomerHistory(ctx context.Context, customerID, restaurantID int64) ([]CustomerHistoryEntry, error) {
	rows, err := s.repos.GetCustomerSessionHistory(ctx, customerID)
	if err != nil {
		return nil, err
	}
	result := make([]CustomerHistoryEntry, len(rows))
	for i, r := range rows {
		var closedAt *string
		if r.ClosedAt.Valid {
			t := r.ClosedAt.Time.String()
			closedAt = &t
		}
		result[i] = CustomerHistoryEntry{
			SessionID:       r.SessionID.String(),
			CreatedAt:       r.CreatedAt.String(),
			ClosedAt:        closedAt,
			TableIdentifier: r.TableIdentifier,
			TotalSpent:      formatTotalSpent(r.TotalSpent),
		}
	}
	return result, nil
}

func (s *CustomerService) DeleteCustomer(ctx context.Context, customerID, restaurantID int64) error {
	return s.repos.DeleteCustomer(ctx, customerID, restaurantID)
}

type CustomerListItem struct {
	ID          int64  `json:"id"`
	PhoneMasked string `json:"phone_masked"`
	DisplayName string `json:"display_name"`
	VisitCount  int    `json:"visit_count"`
	LastSeenAt  string `json:"last_seen_at"`
	OptedIn     bool   `json:"opted_in"`
}

type CustomerHistoryEntry struct {
	SessionID       string  `json:"session_id"`
	CreatedAt       string  `json:"created_at"`
	ClosedAt        *string `json:"closed_at"`
	TableIdentifier string  `json:"table_identifier"`
	TotalSpent      string  `json:"total_spent"`
}

func maskPhone(phone string) string {
	if len(phone) < 8 {
		return "****"
	}
	return phone[:3] + " ****" + phone[len(phone)-4:]
}

func formatTotalSpent(v interface{}) string {
	if v == nil {
		return "0"
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "0"
		}
		s := strings.Trim(string(b), `"`)
		return s
	}
}

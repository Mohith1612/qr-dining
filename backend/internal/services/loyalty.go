package services

import (
	"context"
	"errors"
	"math/big"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/observability"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

// Loyalty transaction types (ledger rows are append-only, points signed).
const (
	LoyaltyTxnEarn       = "earn"
	LoyaltyTxnRedeem     = "redeem"
	LoyaltyTxnAdjustment = "adjustment"
)

const loyaltyEarnIdempotencyIndex = "idx_loyalty_txn_earn_payment"

// LoyaltyService implements the Phase-1 loyalty foundation: spend → points.
//
// Earn rides the payment-completion paths but is fully error-isolated — a
// loyalty failure can never fail a payment. Redemption is LEDGER-ONLY: staff
// record a points deduction and apply any discount off-system; bill/payment
// math is untouched.
//
// Every surface is gated by the loyalty.* entitlements + the loyalty platform
// flag (all default off), so the whole subsystem is inert until a platform
// operator deliberately enables it for an org.
type LoyaltyService struct {
	repos   *repository.Repos
	gate    *FeatureGate
	cache   *redisPkg.Cache
	logger  zerolog.Logger
	metrics *observability.Metrics
}

func NewLoyaltyService(repos *repository.Repos, gate *FeatureGate, cache *redisPkg.Cache, logger zerolog.Logger, metrics *observability.Metrics) *LoyaltyService {
	return &LoyaltyService{repos: repos, gate: gate, cache: cache, logger: logger, metrics: metrics}
}

// ── Earn (payment-completion hook) ───────────────────────────────────────────

// AccruePointsForPayment is called after a payment reaches 'completed' (both
// webhook and staff-settle paths). It never returns an error and never panics
// the caller's flow: any failure is logged + counted, and an idempotent replay
// (same payment accrued twice) is silently a no-op.
func (s *LoyaltyService) AccruePointsForPayment(ctx context.Context, payment sqlc.Payment) {
	err := s.accrue(ctx, payment)
	if err == nil {
		return
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == loyaltyEarnIdempotencyIndex {
		return // replay of an already-accrued payment
	}
	s.logger.Warn().Err(err).Int64("payment_id", payment.ID).Str("session_id", payment.SessionID.String()).Msg("loyalty accrual failed")
	if s.metrics != nil && s.metrics.LoyaltyAccrualFailuresTotal != nil {
		s.metrics.LoyaltyAccrualFailuresTotal.Inc()
	}
}

func (s *LoyaltyService) accrue(ctx context.Context, payment sqlc.Payment) error {
	enabled, err := s.gate.BranchEnabled(ctx, payment.BranchID, EntitlementLoyaltyEnabled, FlagLoyalty)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	sess, err := s.repos.GetSessionByID(ctx, payment.SessionID)
	if err != nil {
		return err
	}
	if !sess.CustomerID.Valid {
		return nil // anonymous table — nothing to accrue
	}
	customerID := sess.CustomerID.Int64

	branch, err := s.repos.GetBranchByID(ctx, payment.BranchID)
	if err != nil {
		return err
	}
	orgID := branch.OrganizationID

	program, err := s.repos.GetLoyaltyProgram(ctx, orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // org never configured a program
	}
	if err != nil {
		return err
	}
	if !program.IsActive {
		return nil
	}

	points := computeEarnPoints(payment.Amount, program.EarnRateAmount, program.EarnRatePoints)
	if points <= 0 {
		return nil
	}

	return s.repos.WithTx(ctx, func(tx *repository.Repos) error {
		visit := int64(1)
		acct, err := tx.GetLoyaltyAccountByCustomer(ctx, customerID, orgID)
		switch {
		case err == nil:
			n, err := tx.CountEarnTransactionsForAccountSession(ctx, acct.ID, payment.SessionID)
			if err != nil {
				return err
			}
			if n > 0 {
				visit = 0 // second earn in the same session is not a new visit
			}
		case errors.Is(err, pgx.ErrNoRows):
			// first earn ever → account created below, visit = 1
		default:
			return err
		}

		account, err := tx.UpsertLoyaltyAccountForEarn(ctx, customerID, orgID, points, visit, payment.Amount)
		if err != nil {
			return err
		}
		_, err = tx.InsertLoyaltyTransaction(ctx, repository.LoyaltyTransactionParams{
			AccountID: account.ID,
			Type:      LoyaltyTxnEarn,
			Points:    points,
			Amount:    payment.Amount,
			PaymentID: payment.ID,
			SessionID: payment.SessionID,
			ActorType: "system",
		})
		return err
	})
}

// computeEarnPoints returns floor(amount * ratePoints / rateAmount) using
// exact rational arithmetic — no float drift on money values. Any unparseable
// input yields 0 (no accrual).
func computeEarnPoints(amount, rateAmount pgtype.Numeric, ratePoints int64) int64 {
	if ratePoints <= 0 {
		return 0
	}
	amt, ok := new(big.Rat).SetString(numericString(amount))
	if !ok || amt.Sign() <= 0 {
		return 0
	}
	rate, ok := new(big.Rat).SetString(numericString(rateAmount))
	if !ok || rate.Sign() <= 0 {
		return 0
	}
	r := new(big.Rat).Mul(amt, big.NewRat(ratePoints, 1))
	r.Quo(r, rate)
	return new(big.Int).Quo(r.Num(), r.Denom()).Int64() // truncation == floor for non-negative
}

// ── Gating ───────────────────────────────────────────────────────────────────

// checkBranchGate resolves the branch's org and enforces the given loyalty
// capability + the loyalty flag at branch scope (so branch-level flag
// overrides behave consistently across every loyalty surface).
func (s *LoyaltyService) checkBranchGate(ctx context.Context, branchID int64, entKey string) (orgID int64, err error) {
	branch, err := s.repos.GetBranchByID(ctx, branchID)
	if err != nil {
		return 0, err
	}
	enabled, err := s.gate.BranchEnabled(ctx, branchID, entKey, FlagLoyalty)
	if err != nil {
		return 0, err
	}
	if !enabled {
		return 0, domain.ErrLoyaltyDisabled
	}
	return branch.OrganizationID, nil
}

// ── Program config ───────────────────────────────────────────────────────────

// LoyaltyProgramDTO is the org-facing earning rule: earn_rate_points per
// earn_rate_amount of spend.
type LoyaltyProgramDTO struct {
	OrganizationID int64  `json:"organization_id"`
	IsActive       bool   `json:"is_active"`
	EarnRatePoints int64  `json:"earn_rate_points"`
	EarnRateAmount string `json:"earn_rate_amount"`
	Configured     bool   `json:"configured"` // false = defaults, no row yet
}

func (s *LoyaltyService) GetProgram(ctx context.Context, branchID int64) (LoyaltyProgramDTO, error) {
	orgID, err := s.checkBranchGate(ctx, branchID, EntitlementLoyaltyEnabled)
	if err != nil {
		return LoyaltyProgramDTO{}, err
	}
	program, err := s.repos.GetLoyaltyProgram(ctx, orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return LoyaltyProgramDTO{OrganizationID: orgID, IsActive: false, EarnRatePoints: 1, EarnRateAmount: "100.00"}, nil
	}
	if err != nil {
		return LoyaltyProgramDTO{}, err
	}
	return programDTO(program), nil
}

func (s *LoyaltyService) PutProgram(ctx context.Context, branchID int64, isActive bool, earnRatePoints int64, earnRateAmount string, staffID int64) (LoyaltyProgramDTO, error) {
	orgID, err := s.checkBranchGate(ctx, branchID, EntitlementLoyaltyEnabled)
	if err != nil {
		return LoyaltyProgramDTO{}, err
	}
	if earnRatePoints <= 0 {
		return LoyaltyProgramDTO{}, domain.ErrInvalidLoyaltyRequest
	}
	amount, err := parseNumeric(earnRateAmount)
	if err != nil {
		return LoyaltyProgramDTO{}, domain.ErrInvalidLoyaltyRequest
	}
	if amt, ok := new(big.Rat).SetString(numericString(amount)); !ok || amt.Sign() <= 0 {
		return LoyaltyProgramDTO{}, domain.ErrInvalidLoyaltyRequest
	}
	program, err := s.repos.UpsertLoyaltyProgram(ctx, orgID, isActive, earnRatePoints, amount, staffID)
	if err != nil {
		return LoyaltyProgramDTO{}, err
	}
	return programDTO(program), nil
}

func programDTO(p sqlc.OrganizationLoyaltyProgram) LoyaltyProgramDTO {
	return LoyaltyProgramDTO{
		OrganizationID: p.OrganizationID,
		IsActive:       p.IsActive,
		EarnRatePoints: p.EarnRatePoints,
		EarnRateAmount: numericString(p.EarnRateAmount),
		Configured:     true,
	}
}

// ── Account lookup / ledger ──────────────────────────────────────────────────

type LoyaltyAccountDTO struct {
	AccountID              int64  `json:"account_id"`
	CustomerID             int64  `json:"customer_id"`
	PointsBalance          int64  `json:"points_balance"`
	LifetimePointsEarned   int64  `json:"lifetime_points_earned"`
	LifetimePointsRedeemed int64  `json:"lifetime_points_redeemed"`
	VisitCount             int64  `json:"visit_count"`
	LifetimeSpend          string `json:"lifetime_spend"`
}

type LoyaltyCustomerLookup struct {
	CustomerID  int64              `json:"customer_id"`
	DisplayName string             `json:"display_name"`
	PhoneE164   string             `json:"phone_e164"`
	Account     *LoyaltyAccountDTO `json:"account"` // nil = no loyalty activity yet
}

type LoyaltyTransactionDTO struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Points    int64     `json:"points"`
	Amount    string    `json:"amount"`
	PaymentID *int64    `json:"payment_id,omitempty"`
	SessionID *string   `json:"session_id,omitempty"`
	ActorType string    `json:"performed_by_actor_type"`
	StaffID   *int64    `json:"performed_by_staff_id,omitempty"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// LookupCustomer finds a customer by phone within the branch's restaurant and
// attaches the loyalty account when one exists.
func (s *LoyaltyService) LookupCustomer(ctx context.Context, branchID int64, phone string) (LoyaltyCustomerLookup, error) {
	orgID, err := s.checkBranchGate(ctx, branchID, EntitlementLoyaltyEnabled)
	if err != nil {
		return LoyaltyCustomerLookup{}, err
	}
	normalized, err := normalizePhone(phone)
	if err != nil {
		return LoyaltyCustomerLookup{}, err
	}
	restaurant, err := s.repos.GetRestaurantByOrganizationID(ctx, orgID)
	if err != nil {
		return LoyaltyCustomerLookup{}, err
	}
	customer, err := s.repos.GetCustomerByPhone(ctx, restaurant.ID, normalized)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LoyaltyCustomerLookup{}, domain.ErrCustomerNotFound
		}
		return LoyaltyCustomerLookup{}, err
	}
	out := LoyaltyCustomerLookup{CustomerID: customer.ID, DisplayName: customer.DisplayName, PhoneE164: customer.PhoneE164}
	account, err := s.repos.GetLoyaltyAccountByCustomer(ctx, customer.ID, orgID)
	if err == nil {
		dto := accountDTO(account)
		out.Account = &dto
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return LoyaltyCustomerLookup{}, err
	}
	return out, nil
}

// ListTransactions returns the ledger for an account (org-isolated).
func (s *LoyaltyService) ListTransactions(ctx context.Context, branchID, accountID int64, limit, offset int32) ([]LoyaltyTransactionDTO, error) {
	orgID, err := s.checkBranchGate(ctx, branchID, EntitlementLoyaltyEnabled)
	if err != nil {
		return nil, err
	}
	if _, err := s.repos.GetLoyaltyAccountByID(ctx, accountID, orgID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrLoyaltyAccountNotFound
		}
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.repos.ListLoyaltyTransactions(ctx, accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]LoyaltyTransactionDTO, 0, len(rows))
	for _, t := range rows {
		out = append(out, transactionDTO(t))
	}
	return out, nil
}

func accountDTO(a sqlc.CustomerLoyaltyAccount) LoyaltyAccountDTO {
	return LoyaltyAccountDTO{
		AccountID:              a.ID,
		CustomerID:             a.CustomerID,
		PointsBalance:          a.PointsBalance,
		LifetimePointsEarned:   a.LifetimePointsEarned,
		LifetimePointsRedeemed: a.LifetimePointsRedeemed,
		VisitCount:             a.VisitCount,
		LifetimeSpend:          numericString(a.LifetimeSpend),
	}
}

func transactionDTO(t sqlc.CustomerLoyaltyTransaction) LoyaltyTransactionDTO {
	dto := LoyaltyTransactionDTO{
		ID:        t.ID,
		Type:      t.Type,
		Points:    t.Points,
		Amount:    numericString(t.Amount),
		ActorType: t.PerformedByActorType,
		Reason:    t.Reason,
		CreatedAt: t.CreatedAt,
	}
	if t.PaymentID.Valid {
		dto.PaymentID = &t.PaymentID.Int64
	}
	if t.SessionID.Valid {
		id := uuid.UUID(t.SessionID.Bytes).String()
		dto.SessionID = &id
	}
	if t.PerformedByStaffID.Valid {
		dto.StaffID = &t.PerformedByStaffID.Int64
	}
	return dto
}

// ── Redeem / adjust (ledger-only) ────────────────────────────────────────────

// Redeem records a staff-witnessed points deduction. It does NOT touch any
// bill or payment — the corresponding discount (if any) is applied off-system.
func (s *LoyaltyService) Redeem(ctx context.Context, branchID, accountID, points int64, reason string, staffID int64) (LoyaltyAccountDTO, error) {
	orgID, err := s.checkBranchGate(ctx, branchID, EntitlementLoyaltyEnabled)
	if err != nil {
		return LoyaltyAccountDTO{}, err
	}
	if _, err := s.checkBranchGate(ctx, branchID, EntitlementLoyaltyRedeem); err != nil {
		return LoyaltyAccountDTO{}, err
	}
	if points <= 0 {
		return LoyaltyAccountDTO{}, domain.ErrInvalidLoyaltyRequest
	}
	var updated sqlc.CustomerLoyaltyAccount
	err = s.repos.WithTx(ctx, func(tx *repository.Repos) error {
		if _, err := tx.GetLoyaltyAccountByID(ctx, accountID, orgID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrLoyaltyAccountNotFound
			}
			return err
		}
		acct, err := tx.DeductLoyaltyPoints(ctx, accountID, orgID, points)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrLoyaltyInsufficientPoints
		}
		if err != nil {
			return err
		}
		if _, err := tx.InsertLoyaltyTransaction(ctx, repository.LoyaltyTransactionParams{
			AccountID: accountID,
			Type:      LoyaltyTxnRedeem,
			Points:    -points,
			ActorType: "staff",
			StaffID:   staffID,
			Reason:    reason,
		}); err != nil {
			return err
		}
		updated = acct
		return nil
	})
	if err != nil {
		return LoyaltyAccountDTO{}, err
	}
	return accountDTO(updated), nil
}

// Adjust records a signed manual correction (owner/manager only at the API
// layer, gated by loyalty.manual_adjustment). Floors at zero balance.
func (s *LoyaltyService) Adjust(ctx context.Context, branchID, accountID, pointsDelta int64, reason string, staffID int64) (LoyaltyAccountDTO, error) {
	orgID, err := s.checkBranchGate(ctx, branchID, EntitlementLoyaltyEnabled)
	if err != nil {
		return LoyaltyAccountDTO{}, err
	}
	if _, err := s.checkBranchGate(ctx, branchID, EntitlementLoyaltyManualAdjustment); err != nil {
		return LoyaltyAccountDTO{}, err
	}
	if pointsDelta == 0 || reason == "" {
		return LoyaltyAccountDTO{}, domain.ErrInvalidLoyaltyRequest
	}
	var updated sqlc.CustomerLoyaltyAccount
	err = s.repos.WithTx(ctx, func(tx *repository.Repos) error {
		if _, err := tx.GetLoyaltyAccountByID(ctx, accountID, orgID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrLoyaltyAccountNotFound
			}
			return err
		}
		acct, err := tx.AdjustLoyaltyPoints(ctx, accountID, orgID, pointsDelta)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrLoyaltyInsufficientPoints
		}
		if err != nil {
			return err
		}
		if _, err := tx.InsertLoyaltyTransaction(ctx, repository.LoyaltyTransactionParams{
			AccountID: accountID,
			Type:      LoyaltyTxnAdjustment,
			Points:    pointsDelta,
			ActorType: "staff",
			StaffID:   staffID,
			Reason:    reason,
		}); err != nil {
			return err
		}
		updated = acct
		return nil
	})
	if err != nil {
		return LoyaltyAccountDTO{}, err
	}
	return accountDTO(updated), nil
}

// ── Analytics ────────────────────────────────────────────────────────────────

type LoyaltyTopCustomer struct {
	AccountID            int64  `json:"account_id"`
	CustomerID           int64  `json:"customer_id"`
	DisplayName          string `json:"display_name"`
	PhoneE164            string `json:"phone_e164"`
	PointsBalance        int64  `json:"points_balance"`
	LifetimePointsEarned int64  `json:"lifetime_points_earned"`
	VisitCount           int64  `json:"visit_count"`
	LifetimeSpend        string `json:"lifetime_spend"`
}

type LoyaltyAnalytics struct {
	Period          string               `json:"period"`
	PointsIssued    int64                `json:"points_issued"`
	PointsRedeemed  int64                `json:"points_redeemed"`
	EarnCount       int64                `json:"earn_count"`
	RedeemCount     int64                `json:"redeem_count"`
	ActiveCustomers int64                `json:"active_customers"`
	TotalAccounts   int64                `json:"total_accounts"`
	TotalSessions   int64                `json:"total_sessions"`
	LoyaltySessions int64                `json:"loyalty_sessions"`
	TopCustomers    []LoyaltyTopCustomer `json:"top_customers"`
}

// GetLoyaltyAnalytics returns org-level participation, points issued/redeemed,
// and top customers for the window (accounts are org-scoped; the branch is the
// access scope).
func (s *LoyaltyService) GetLoyaltyAnalytics(ctx context.Context, branchID int64, period string) (LoyaltyAnalytics, error) {
	orgID, err := s.checkBranchGate(ctx, branchID, EntitlementLoyaltyEnabled)
	if err != nil {
		return LoyaltyAnalytics{}, err
	}
	key := orgCacheKey(orgID, period, "loyalty")
	var cached LoyaltyAnalytics
	if s.cache != nil {
		if hit, _ := s.cache.Get(ctx, key, &cached); hit {
			return cached, nil
		}
	}
	from, to := periodWindow(period)
	summary, err := s.repos.GetLoyaltyPointsSummary(ctx, orgID, from, to)
	if err != nil {
		return LoyaltyAnalytics{}, err
	}
	participation, err := s.repos.GetLoyaltyParticipation(ctx, orgID, from, to)
	if err != nil {
		return LoyaltyAnalytics{}, err
	}
	totalAccounts, err := s.repos.CountLoyaltyAccounts(ctx, orgID)
	if err != nil {
		return LoyaltyAnalytics{}, err
	}
	topRows, err := s.repos.GetTopLoyaltyCustomers(ctx, orgID)
	if err != nil {
		return LoyaltyAnalytics{}, err
	}
	top := make([]LoyaltyTopCustomer, 0, len(topRows))
	for _, t := range topRows {
		top = append(top, LoyaltyTopCustomer{
			AccountID:            t.AccountID,
			CustomerID:           t.CustomerID,
			DisplayName:          t.DisplayName,
			PhoneE164:            t.PhoneE164,
			PointsBalance:        t.PointsBalance,
			LifetimePointsEarned: t.LifetimePointsEarned,
			VisitCount:           t.VisitCount,
			LifetimeSpend:        numericString(t.LifetimeSpend),
		})
	}
	report := LoyaltyAnalytics{
		Period:          period,
		PointsIssued:    summary.PointsIssued,
		PointsRedeemed:  summary.PointsRedeemed,
		EarnCount:       summary.EarnCount,
		RedeemCount:     summary.RedeemCount,
		ActiveCustomers: summary.ActiveCustomers,
		TotalAccounts:   totalAccounts,
		TotalSessions:   participation.TotalSessions,
		LoyaltySessions: participation.LoyaltySessions,
		TopCustomers:    top,
	}
	if s.cache != nil {
		_ = s.cache.Set(ctx, key, report, analyticsCacheTTL)
	}
	return report, nil
}

// Error predicates for handler mapping.
func IsLoyaltyDisabled(err error) bool { return errors.Is(err, domain.ErrLoyaltyDisabled) }
func IsLoyaltyInsufficientPoints(err error) bool {
	return errors.Is(err, domain.ErrLoyaltyInsufficientPoints)
}
func IsLoyaltyAccountNotFound(err error) bool {
	return errors.Is(err, domain.ErrLoyaltyAccountNotFound)
}

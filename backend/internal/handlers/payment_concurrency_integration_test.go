//go:build integration

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/observability"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

func TestSettlePaymentTwice_SecondRefusedAndCollectedOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)
	ctx := context.Background()
	sess, _, payment := f.freezeSession(t, f.a, sqlc.PaymentStatusRequiresStaffConfirmation)

	if _, err := f.paymentSvc.SettlePaymentByStaff(ctx, payment.ID, f.a.waiter.StaffID, f.a.branchID); err != nil {
		t.Fatalf("first settlement: %v", err)
	}
	if _, err := f.paymentSvc.SettlePaymentByStaff(ctx, payment.ID, f.a.waiter.StaffID, f.a.branchID); !errors.Is(err, domain.ErrInvalidPaymentTransition) {
		t.Fatalf("second settlement error: got %v, want ErrInvalidPaymentTransition", err)
	}

	var collected float64
	if err := f.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM payments WHERE session_id = $1 AND status = 'completed'`,
		sess.Session.ID,
	).Scan(&collected); err != nil {
		t.Fatalf("sum completed payments: %v", err)
	}
	if collected != 50 {
		t.Fatalf("completed payment total: got %.2f, want 50.00", collected)
	}
}

func TestCancelPayment_AllowsDistinctNewPayment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)
	ctx := context.Background()
	sess, _, first := f.freezeSession(t, f.a, sqlc.PaymentStatusRequiresStaffConfirmation)

	if _, err := f.paymentSvc.CancelPaymentByStaff(ctx, first.ID, f.a.waiter.StaffID, f.a.branchID, "guest changed method"); err != nil {
		t.Fatalf("CancelPaymentByStaff: %v", err)
	}
	if got := f.sessionStatus(t, sess.Session.ID); got != string(sqlc.SessionStatusActive) {
		t.Fatalf("session status after cancel: got %s, want active", got)
	}

	second, err := f.paymentSvc.InitiatePayment(ctx, services.InitiatePaymentRequest{
		SessionID:      sess.Session.ID,
		BranchID:       f.a.branchID,
		Method:         sqlc.PaymentMethodCard,
		IdempotencyKey: uuid.NewString(),
		ActorType:      "participant",
		ActorID:        sess.Participant.ID,
		Bill: services.BillSnapshotInput{
			Subtotal:       50,
			Total:          50,
			Currency:       "INR",
			CreatedByActor: "guest",
		},
	})
	if err != nil {
		t.Fatalf("second InitiatePayment: %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("payment after cancel reused ID %d", first.ID)
	}
	if second.Status != sqlc.PaymentStatusRequiresStaffConfirmation {
		t.Fatalf("second payment status: got %s, want requires_staff_confirmation", second.Status)
	}
}

func TestPaymentSettlement_ClosesOnlyWhenItsBillSnapshotIsFullyPaid(t *testing.T) {
	t.Run("one completed full-total payment closes the session", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		f := newRecoveryFixture(t)
		ctx := context.Background()
		sess, _ := f.openSession(t, f.a)

		payment, err := f.paymentSvc.InitiatePayment(ctx, services.InitiatePaymentRequest{
			SessionID:      sess.Session.ID,
			BranchID:       f.a.branchID,
			Method:         sqlc.PaymentMethodCash,
			IdempotencyKey: uuid.NewString(),
			ActorType:      "participant",
			ActorID:        sess.Participant.ID,
			Bill: services.BillSnapshotInput{
				Subtotal:       300,
				Total:          300,
				Currency:       "INR",
				CreatedByActor: "guest",
			},
		})
		if err != nil {
			t.Fatalf("InitiatePayment: %v", err)
		}
		if _, err := f.paymentSvc.SettlePaymentByStaff(ctx, payment.ID, f.a.waiter.StaffID, f.a.branchID); err != nil {
			t.Fatalf("SettlePaymentByStaff: %v", err)
		}
		if got := f.sessionStatus(t, sess.Session.ID); got != string(sqlc.SessionStatusClosed) {
			t.Fatalf("session status: got %s, want closed", got)
		}
	})

	t.Run("completed money from a superseded snapshot does not settle the current snapshot", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		f := newRecoveryFixture(t)
		ctx := context.Background()

		if _, err := f.pool.Exec(ctx, `UPDATE menu_items SET price = 300.00 WHERE id = $1`, f.a.itemID); err != nil {
			t.Fatalf("set menu item price: %v", err)
		}
		sess, _ := f.openSession(t, f.a)
		orderSvc := services.NewOrderService(f.repos, f.publisher, observability.NewMetrics(), services.NewPromoService(f.repos))
		orderSvc.SetHostAuthority(f.sessionSvc)
		placed, err := orderSvc.PlaceOrder(ctx, services.PlaceOrderRequest{
			SessionID:             sess.Session.ID,
			BranchID:              f.a.branchID,
			PlacedByParticipantID: sess.Participant.ID,
			IdempotencyKey:        uuid.NewString(),
			Items:                 []services.OrderItem{{MenuItemID: f.a.itemID, Quantity: 1}},
		})
		if err != nil {
			t.Fatalf("PlaceOrder: %v", err)
		}
		sourceOrderIDs, err := json.Marshal([]string{placed.Order.ID.String()})
		if err != nil {
			t.Fatalf("marshal source order ids: %v", err)
		}

		var supersededSnapshotID, currentSnapshotID int64
		if err := f.pool.QueryRow(ctx, `
			INSERT INTO bill_snapshots
			    (session_id, branch_id, subtotal, total, currency, source_order_ids, created_by_actor, created_at)
			VALUES ($1, $2, 150.00, 150.00, 'INR', $3, 'legacy', now() - interval '1 minute')
			RETURNING id`, sess.Session.ID, f.a.branchID, sourceOrderIDs).Scan(&supersededSnapshotID); err != nil {
			t.Fatalf("insert superseded snapshot: %v", err)
		}
		if err := f.pool.QueryRow(ctx, `
			INSERT INTO bill_snapshots
			    (session_id, branch_id, subtotal, total, currency, source_order_ids, created_by_actor)
			VALUES ($1, $2, 300.00, 300.00, 'INR', $3, 'guest')
			RETURNING id`, sess.Session.ID, f.a.branchID, sourceOrderIDs).Scan(&currentSnapshotID); err != nil {
			t.Fatalf("insert current snapshot: %v", err)
		}

		if _, err := f.pool.Exec(ctx, `
			INSERT INTO payments
			    (session_id, amount, method, status, completed_at, bill_snapshot_id, branch_id,
			     currency, payment_business_date, payment_sequence, payment_reference)
			VALUES ($1, 150.00, 'cash', 'completed', now() - interval '1 minute', $2, $3,
			        'INR', current_date, 1, $4)`,
			sess.Session.ID, supersededSnapshotID, f.a.branchID, "OLD-"+uuid.NewString()); err != nil {
			t.Fatalf("insert completed superseded payment: %v", err)
		}
		var currentPaymentID int64
		if err := f.pool.QueryRow(ctx, `
			INSERT INTO payments
			    (session_id, amount, method, status, bill_snapshot_id, branch_id,
			     currency, payment_business_date, payment_sequence, payment_reference)
			VALUES ($1, 150.00, 'cash', 'requires_staff_confirmation', $2, $3,
			        'INR', current_date, 2, $4)
			RETURNING id`,
			sess.Session.ID, currentSnapshotID, f.a.branchID, "CURRENT-"+uuid.NewString()).Scan(&currentPaymentID); err != nil {
			t.Fatalf("insert current payment: %v", err)
		}
		if _, err := f.pool.Exec(ctx, `UPDATE sessions SET status = 'payment_pending' WHERE id = $1`, sess.Session.ID); err != nil {
			t.Fatalf("freeze session: %v", err)
		}

		if _, err := f.paymentSvc.SettlePaymentByStaff(ctx, currentPaymentID, f.a.waiter.StaffID, f.a.branchID); err != nil {
			t.Fatalf("SettlePaymentByStaff: %v", err)
		}
		if got := f.sessionStatus(t, sess.Session.ID); got != string(sqlc.SessionStatusPaymentPending) {
			t.Fatalf("session status: got %s, want payment_pending", got)
		}
	})
}

func TestInitiatePayment_RejectsAmountBelowBillTotal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)
	ctx := context.Background()

	if _, err := f.pool.Exec(ctx, `UPDATE menu_items SET price = 300.00 WHERE id = $1`, f.a.itemID); err != nil {
		t.Fatalf("set menu item price: %v", err)
	}
	sess, _ := f.openSession(t, f.a)
	orderSvc := services.NewOrderService(f.repos, f.publisher, observability.NewMetrics(), services.NewPromoService(f.repos))
	orderSvc.SetHostAuthority(f.sessionSvc)
	if _, err := orderSvc.PlaceOrder(ctx, services.PlaceOrderRequest{
		SessionID:             sess.Session.ID,
		BranchID:              f.a.branchID,
		PlacedByParticipantID: sess.Participant.ID,
		IdempotencyKey:        uuid.NewString(),
		Items:                 []services.OrderItem{{MenuItemID: f.a.itemID, Quantity: 1}},
	}); err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := fmt.Sprintf(`{"amount":150,"method":"cash","idempotency_key":%q}`, uuid.NewString())
	c.Request = httptest.NewRequest(http.MethodPost, "/sessions/"+sess.Session.ID.String()+"/payments", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("X-Participant-ID", strconv.FormatInt(sess.Participant.ID, 10))
	c.Params = gin.Params{{Key: "id", Value: sess.Session.ID.String()}}
	f.paymentHandler(false).InitiatePayment(c)
	c.Writer.WriteHeaderNow()

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("partial payment status: got %d, want 422 (body: %s)", rec.Code, strings.TrimSpace(rec.Body.String()))
	}
	var apiErr APIError
	if err := json.Unmarshal(rec.Body.Bytes(), &apiErr); err != nil {
		t.Fatalf("decode partial payment error: %v", err)
	}
	if apiErr.Code != CodePaymentAmountInvalid || apiErr.Message != "payment amount must equal the full bill total" {
		t.Fatalf("partial payment error: got %+v", apiErr)
	}
	var paymentCount int
	if err := f.pool.QueryRow(ctx, `SELECT COUNT(*) FROM payments WHERE session_id = $1`, sess.Session.ID).Scan(&paymentCount); err != nil {
		t.Fatalf("count payments: %v", err)
	}
	if paymentCount != 0 {
		t.Fatalf("payment rows after partial request: got %d, want 0", paymentCount)
	}
}

func TestInitiatePayment_HostRetryReturnsExistingWithOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)
	ctx := context.Background()

	if _, err := f.pool.Exec(ctx, `UPDATE menu_items SET price = 300.00 WHERE id = $1`, f.a.itemID); err != nil {
		t.Fatalf("set menu item price: %v", err)
	}
	sess, _ := f.openSession(t, f.a)
	orderSvc := services.NewOrderService(f.repos, f.publisher, observability.NewMetrics(), services.NewPromoService(f.repos))
	orderSvc.SetHostAuthority(f.sessionSvc)
	if _, err := orderSvc.PlaceOrder(ctx, services.PlaceOrderRequest{
		SessionID:             sess.Session.ID,
		BranchID:              f.a.branchID,
		PlacedByParticipantID: sess.Participant.ID,
		IdempotencyKey:        uuid.NewString(),
		Items:                 []services.OrderItem{{MenuItemID: f.a.itemID, Quantity: 1}},
	}); err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	h := f.paymentHandler(false)
	initiate := func() (*httptest.ResponseRecorder, sqlc.Payment) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		body := fmt.Sprintf(`{"amount":300,"method":"cash","idempotency_key":%q}`, uuid.NewString())
		c.Request = httptest.NewRequest(http.MethodPost, "/sessions/"+sess.Session.ID.String()+"/payments", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Request.Header.Set("X-Participant-ID", strconv.FormatInt(sess.Participant.ID, 10))
		c.Params = gin.Params{{Key: "id", Value: sess.Session.ID.String()}}
		h.InitiatePayment(c)
		c.Writer.WriteHeaderNow()
		var payment sqlc.Payment
		if err := json.Unmarshal(rec.Body.Bytes(), &payment); err != nil {
			t.Fatalf("decode payment response (status %d): %v; body=%s", rec.Code, err, rec.Body.String())
		}
		return rec, payment
	}

	firstResponse, first := initiate()
	if firstResponse.Code != http.StatusCreated {
		t.Fatalf("first initiation status: got %d, want 201", firstResponse.Code)
	}
	secondResponse, second := initiate()
	if secondResponse.Code != http.StatusOK {
		t.Fatalf("retry status: got %d, want 200", secondResponse.Code)
	}
	if second.ID != first.ID {
		t.Fatalf("retry returned payment %d, want existing payment %d", second.ID, first.ID)
	}
}

func TestConcurrentPaymentInitiations_CreateAndCollectOneFullBillPayment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newRecoveryFixture(t)
	ctx := context.Background()

	if _, err := f.pool.Exec(ctx, `UPDATE menu_items SET price = 300.00 WHERE id = $1`, f.a.itemID); err != nil {
		t.Fatalf("set menu item price: %v", err)
	}
	sess, _ := f.openSession(t, f.a)
	secondParticipant, err := f.sessionSvc.JoinSession(ctx, sess.Session.ID, "Second guest", "fp-second", "")
	if err != nil {
		t.Fatalf("JoinSession: %v", err)
	}
	f.markPresent(t, f.a, sess.Session.ID, sess.Participant.ID)
	f.markPresent(t, f.a, sess.Session.ID, secondParticipant.ID)

	orderSvc := services.NewOrderService(f.repos, f.publisher, observability.NewMetrics(), services.NewPromoService(f.repos))
	orderSvc.SetHostAuthority(f.sessionSvc)
	if _, err := orderSvc.PlaceOrder(ctx, services.PlaceOrderRequest{
		SessionID:             sess.Session.ID,
		BranchID:              f.a.branchID,
		PlacedByParticipantID: sess.Participant.ID,
		IdempotencyKey:        uuid.NewString(),
		Items:                 []services.OrderItem{{MenuItemID: f.a.itemID, Quantity: 1}},
	}); err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	bill, err := ComputeBillForSession(ctx, f.repos, sess.Session.ID)
	if err != nil {
		t.Fatalf("ComputeBillForSession: %v", err)
	}
	if bill.Total != 300 {
		t.Fatalf("bill total: got %.2f, want 300.00", bill.Total)
	}

	// Isolate the payment invariant below host authorization. With the real
	// host gate wired, the second participant would be denied before reaching
	// the payment race and the pre-fix implementation would pass trivially.
	paymentSvc := services.NewPaymentService(f.repos, f.publisher, observability.NewMetrics(), f.sessionSvc, zerolog.Nop())
	paymentSvc.SetPromoService(services.NewPromoService(f.repos))
	h := NewPaymentHandler(
		paymentSvc, f.repos, nil, config.FeatureFlags{}, config.PaymentConfig{},
		nil, f.auditW, services.NewPromoService(f.repos),
	)

	initiate := func(participantID int64, amount float64) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		body := fmt.Sprintf(`{"amount":%.2f,"method":"cash","idempotency_key":%q}`, amount, uuid.NewString())
		c.Request = httptest.NewRequest(http.MethodPost, "/sessions/"+sess.Session.ID.String()+"/payments", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Request.Header.Set("X-Participant-ID", strconv.FormatInt(participantID, 10))
		c.Params = gin.Params{{Key: "id", Value: sess.Session.ID.String()}}
		h.InitiatePayment(c)
		c.Writer.WriteHeaderNow()
		return rec
	}

	start := make(chan struct{})
	partialFinished := make(chan struct{})
	responses := make(chan *httptest.ResponseRecorder, 2)
	go func() {
		<-start
		responses <- initiate(sess.Participant.ID, 150)
		close(partialFinished)
	}()
	go func() {
		<-start
		deadline := time.After(5 * time.Second)
		for {
			var status string
			err := f.pool.QueryRow(ctx, `SELECT status FROM sessions WHERE id = $1`, sess.Session.ID).Scan(&status)
			if err != nil {
				responses <- httptest.NewRecorder()
				return
			}
			if status == string(sqlc.SessionStatusPaymentPending) {
				break
			}
			select {
			case <-partialFinished:
				// falls through to the loop exit below
			case <-deadline:
				responses <- httptest.NewRecorder()
				return
			default:
				time.Sleep(time.Millisecond)
				continue
			}
			break
		}
		responses <- initiate(secondParticipant.ID, 300)
	}()
	close(start)
	firstResponse, secondResponse := <-responses, <-responses
	if firstResponse.Code != http.StatusCreated && secondResponse.Code != http.StatusCreated {
		t.Fatalf("neither initiation succeeded: statuses %d and %d", firstResponse.Code, secondResponse.Code)
	}

	rows, err := f.pool.Query(ctx, `SELECT id FROM payments WHERE session_id = $1 ORDER BY id`, sess.Session.ID)
	if err != nil {
		t.Fatalf("list payments: %v", err)
	}
	var paymentIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			t.Fatalf("scan payment: %v", err)
		}
		paymentIDs = append(paymentIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatalf("iterate payments: %v", err)
	}
	rows.Close()
	if len(paymentIDs) != 1 {
		t.Errorf("payment rows: got %d (%v), want exactly 1", len(paymentIDs), paymentIDs)
	}

	for _, paymentID := range paymentIDs {
		if _, err := paymentSvc.SettlePaymentByStaff(ctx, paymentID, f.a.waiter.StaffID, f.a.branchID); err != nil {
			t.Errorf("settle payment %d: %v", paymentID, err)
		}
	}
	var collected float64
	if err := f.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM payments WHERE session_id = $1 AND status = 'completed'`,
		sess.Session.ID,
	).Scan(&collected); err != nil {
		t.Fatalf("sum completed payments: %v", err)
	}
	if collected != 300 {
		t.Errorf("completed payment total: got %.2f, want 300.00", collected)
	}
}

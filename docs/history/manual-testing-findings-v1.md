# Manual Testing Findings V1

## Context

Manual multi-role walkthrough conducted during R1 soak period.

Testing performed across:

* Guest flow
* Waiter flow
* Kitchen flow
* Admin flow
* Mobile viewport testing
* Session lifecycle testing
* Payment flow testing
* Realtime/WebSocket testing

Current status:

* Backend architecture stable
* R1 soak active
* Focus now shifting toward workflow correctness and production UX polish

---

# P0 — Critical Functional / Workflow Issues

## 1. Same Table Multi-User Join Broken

### Issue

Joining the same table/session again using the same table link redirects back to home screen.

### Expected

Multiple guests should be able to join the same active dining session/table simultaneously.

### Impact

Core dining-table collaborative experience broken.

### Priority

P0

---

## 2. 409 Conflict After Session Reactivation

### Issue

After inactivity timeout popup:

* choosing “still ordering”
* then attempting to place order
  returns 409 conflict until full page refresh.

### Expected

Frontend should properly resync/reactivate session without requiring manual refresh.

### Suspected Cause

Frontend/backend session state mismatch during awaiting_reactivation flow.

### Priority

P0

---

## 3. Kitchen Screen Missing Order Details

### Issue

Kitchen dashboard only shows:

* order number
* order stage

Missing:

* ordered items
* modifiers
* notes
* additions
* spice/customization info

### Expected

Kitchen must see full operational order details.

### Impact

Kitchen workflow operationally unusable.

### Priority

P0

---

## 4. Payment Methods Returning 500 Errors

### Issue

Cash/card/UPI flows intermittently returning 500 internal server errors.

### Expected

Payment flow should complete consistently.

### Suspected Cause

Frontend/backend API contract mismatch or stale payload handling.

### Priority

P0

---

# P1 — Operational Workflow Correctness

## 5. Kitchen Should Not Mark Orders As Served

### Current

Kitchen can move order to “served”.

### Expected

Kitchen:

* pending → preparing → ready

Waiter:

* ready → served

### Priority

P1

---

## 6. Cash/Card Payment Workflow Incorrect

### Current

Cash/card immediately settles payment.

### Expected

Guest:

* requests payment

Waiter:

* receives payment request
* collects payment physically
* confirms collection

Then:

* payment marked complete

### Priority

P1

---

## 7. WebSocket Realtime Delay

### Issue

Realtime updates take noticeable seconds to appear across screens.

### Impact

Operational responsiveness reduced.

### Priority

P1

---

## 8. Admin Toggle Causes Permanent “Connection Lost”

### Issue

Disabling menu item from admin causes guest menu to enter persistent connection lost state.

State does not recover even after refresh/re-enable.

### Suspected Cause

WebSocket/session invalidation handling issue.

### Priority

P1

---

# P2 — UX / Product Polish

## 9. Staff Login PIN Alignment

### Issue

6-digit PIN input not visually centered with sign-in button.

### Priority

P2

---

## 10. Beverage Modifier Selection Incorrect

### Issue

Multiple modifier selections allowed within same modifier category.

### Expected

Single-select radio behavior.

### Priority

P2

---

## 11. Promo Code Placement Incorrect

### Current

Promo appears during order placement.

### Expected

Promo should appear during bill payment flow.

### Priority

P2

---

## 12. Ready-To-Send Screen Missing Quantity Controls

### Expected

Allow:

* increment/decrement
* quantity adjustments
  before final send to kitchen.

### Priority

P2

---

## 13. Menu Item Layout Inconsistent

### Current

Menu:

* image right
* circular

Cart/ready screen:

* image left

### Expected

Consistent image-left layout across flows.

### Priority

P2

---

# P3 — Admin / Expansion Features

## 14. Staff List Missing In Admin

### Current

Can add staff but cannot view/manage existing staff list.

### Priority

P3

---

## 15. Table Management Missing

### Missing Features

* add tables
* rename tables
* remove tables
* regenerate QR

### Priority

P3

---

## 16. QR Customization Missing

### Desired

* shape customization
* branding/logo
* printable layouts

### Priority

P3

---

## 17. Platform Dashboard Required

### Desired Platform Features

* create/manage cafes
* plans/features management
* analytics aggregation
* organization oversight
* platform-level metrics
* feature gating

### Priority

P3

---

# Overall Assessment

Key observation:
Most discovered issues are:

* workflow correctness
* operational UX
* frontend consistency

NOT:

* fundamental backend architectural failures.

Backend foundation now appears largely stable and production-oriented.

Recommended next phase:

1. Continue manual testing
2. Batch backend workflow corrections
3. Final backend stabilization pass
4. Frontend production polish sprint
5. Controlled pilot deployment

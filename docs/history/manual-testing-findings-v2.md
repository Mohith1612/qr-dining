# Manual Testing Findings V2

## Context

Second major manual walkthrough conducted after:

* backend workflow stabilization pass
* frontend contract alignment pass

Testing performed across:

* guest flow
* waiter flow
* kitchen flow
* admin flow
* realtime propagation
* collaborative session behavior
* payment lifecycle behavior
* multi-user table behavior

Important:
Many V1 issues are now resolved or partially resolved.

This V2 focuses primarily on:

* operational workflow gaps
* collaborative session semantics
* realtime coordination behavior
* remaining frontend UX gaps

---

# P0 — Operational / Realtime Gaps

## 1. Waiter Not Receiving Ready-To-Serve Notifications

### Current

Kitchen can:

* acknowledge
* prepare
* mark ready

However:
ready-to-serve updates are not reaching waiter workflow.

### Impact

Orders become stuck operationally at:
`ready`

Guest never sees:
`served`

### Important Observation

Waiter assist requests ARE reaching waiter successfully.

This strongly suggests:

* waiter branch subscription is functioning
* waiter authentication is functioning
* waiter targeting is functioning

Likely root issue:

* ready-to-serve events not emitted
* OR waiter UI not subscribing/rendering them
* OR waiter polling excludes ready-stage orders

### Priority

P0

---

## 2. Payment Requests Not Reaching Waiter

### Current

Guest payment requests:

* cash
* card
* UPI

remain:
`awaiting confirmation`

but waiter never receives operational notification.

### Important Observation

This likely shares the same root issue category as P0-1.

Waiter-assist requests still work correctly.

### Priority

P0

---

## 3. Guest Order State Not Updating Reliably In Realtime

### Current

Guest often must refresh page to see:

* cooking
* ready
* served

status transitions.

### Impact

Realtime experience feels stale/untrustworthy.

### Suspected Areas

* guest websocket reconciliation
* order status propagation
* stale local state
* delayed polling fallback

### Priority

P0

---

# P1 — Collaborative Session Semantics

## 4. Collaborative Ordering Ownership Undefined

### Current

Any participant joining the table can:

* independently submit orders
* independently place to kitchen

### Problem

This creates:

* ordering chaos
* duplicate submissions
* accidental ordering
* unclear operational ownership

### Proposed Product Model

#### Host

First participant joining session becomes:
`session host`

Host controls:

* sending orders to kitchen
* final cart submission
* potentially bill request authority

#### Other Participants

Can:

* add/edit cart items
* collaborate

Cannot:

* independently submit orders

### Important

This is NOT a bug.
This is a product/workflow design decision.

### Priority

P1

---

## 5. Participant Count Incorrect

### Current

Guest participant count remains:
`1`

even with multiple joined participants.

### Priority

P1

---

## 6. Session Rejoin Conflict Edge Case

### Current

When session remains active:

* bill requested
* table not fully closed

new participant join flow sometimes:

* claims new session creation
* then throws 409 conflict

### Priority

P1

---

# P2 — Frontend / UX Gaps

## 7. Kitchen Screen Still Missing Item Rendering

### Current

Kitchen still only displays:

* order number
* stage

Backend now correctly supplies:

* items
* modifiers
* notes

Frontend rendering still incomplete.

### Priority

P2

---

## 8. Modifier Groups Still Allow Multi-Select

### Current

Single-choice beverage modifiers still allow multiple selections.

### Expected

Radio-style single-select behavior.

### Priority

P2

---

## 9. Menu Image Alignment Still Inconsistent

### Current

Menu images still appear:

* right-aligned
* circular

Desired:

* left-aligned consistency
* unified menu/cart visual structure

### Priority

P2

---

## 10. Inactivity Popup Too Aggressive

### Current

"Still here?" popup appears roughly every 30 seconds.

Appears even while:

* browsing
* deciding
* interacting slowly

### Impact

Feels disruptive and stressful.

### Likely Cause

Activity detection incomplete:

* scrolling
* typing
* touch interactions
* cart edits

may not reset inactivity timer.

### Priority

P2

---

# P3 — Admin / Product Expansion

## 11. Staff Details Missing In Admin

### Current

Cannot view/manage existing staff members.

### Priority

P3

---

## 12. Table Management Missing

### Missing

* add tables
* remove tables
* regenerate QR
* manage table metadata

### Priority

P3

---

## 13. QR Customization Missing

### Missing

* branding
* logo
* printable variants
* customization

### Priority

P3

---

# Product Suggestions

## Optional Phone Number During Session Join

### Proposed Join Flow

Required:

* display name

Optional:

* phone number

With:
`Proceed without phone number`

option available.

### Benefits

* clearer participant identification
* easier waiter/kitchen coordination
* better collaborative ordering visibility
* future loyalty/CRM possibilities
* less confusion than Guest 1 / Guest 2

### Important

Should remain optional to preserve low-friction onboarding.

---

# Overall Assessment

Current system status:

* backend architecture stable
* frontend architecture stable
* core lifecycle stable
* auth/tenancy stable
* realtime foundation functioning

Remaining gaps are now primarily:

* operational coordination
* collaborative session semantics
* realtime propagation consistency
* frontend UX refinement
* product workflow definition

System now feels much closer to:
pilot-grade operational software
than:
an unstable development prototype.

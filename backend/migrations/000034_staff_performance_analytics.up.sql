-- Staff performance analytics: gating seeds + query support.
-- Additive only. No new event capture — staff performance is derived from the
-- existing event_log (ORDER_STATUS_CHANGED / ASSISTANCE_*), payments
-- (settled_by_staff_id), and staff_sessions rows.

-- Composite index for windowed per-branch event scans
-- (event_log so far has only single-column indexes; analytics queries filter
-- on branch_id + event_type + created_at window).
CREATE INDEX idx_event_log_branch_type_created
    ON event_log(branch_id, event_type, created_at);

-- Entitlement catalog seed. Capability is granted to NO plan by default:
-- it only resolves true via an explicit plan_entitlements row or an
-- organization_entitlement_overrides row.
INSERT INTO entitlements (key, kind, description) VALUES
    ('analytics.staff_performance', 'capability', 'Staff performance analytics surfaces (waiter/kitchen/manager)');

-- Feature-flag catalog seed. Deliberate deviation from 000030 (which seeded no
-- product flags): seeding the gate key here guarantees it always exists, so
-- gate resolution is deterministic (default_enabled = FALSE → disabled
-- everywhere until a platform operator sets an override).
INSERT INTO platform_feature_flags (key, name, description, default_enabled) VALUES
    ('staff_performance_analytics', 'Staff performance analytics',
     'Branch/org staff performance dashboards', FALSE);

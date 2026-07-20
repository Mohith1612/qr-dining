DELETE FROM platform_flag_branch_overrides       WHERE flag_key = 'staff_performance_analytics';
DELETE FROM platform_flag_organization_overrides WHERE flag_key = 'staff_performance_analytics';
DELETE FROM platform_flag_global_overrides       WHERE flag_key = 'staff_performance_analytics';
DELETE FROM platform_feature_flags               WHERE key      = 'staff_performance_analytics';

DELETE FROM organization_entitlement_overrides WHERE entitlement_key = 'analytics.staff_performance';
DELETE FROM plan_entitlements                  WHERE entitlement_key = 'analytics.staff_performance';
DELETE FROM entitlements                       WHERE key             = 'analytics.staff_performance';

DROP INDEX IF EXISTS idx_event_log_branch_type_created;

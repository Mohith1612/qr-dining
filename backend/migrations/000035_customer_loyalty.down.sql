DELETE FROM platform_flag_branch_overrides       WHERE flag_key = 'loyalty';
DELETE FROM platform_flag_organization_overrides WHERE flag_key = 'loyalty';
DELETE FROM platform_flag_global_overrides       WHERE flag_key = 'loyalty';
DELETE FROM platform_feature_flags               WHERE key      = 'loyalty';

DELETE FROM organization_entitlement_overrides WHERE entitlement_key IN ('loyalty.enabled', 'loyalty.redeem', 'loyalty.manual_adjustment');
DELETE FROM plan_entitlements                  WHERE entitlement_key IN ('loyalty.enabled', 'loyalty.redeem', 'loyalty.manual_adjustment');
DELETE FROM entitlements                       WHERE key             IN ('loyalty.enabled', 'loyalty.redeem', 'loyalty.manual_adjustment');

DROP TABLE IF EXISTS customer_loyalty_transactions;
DROP TABLE IF EXISTS customer_loyalty_accounts;
DROP TABLE IF EXISTS organization_loyalty_programs;

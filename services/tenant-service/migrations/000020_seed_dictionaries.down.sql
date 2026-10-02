-- Link configuration must be removed before dictionary/master tables.
-- The seeded parent rows are intentionally left for their owning table-down
-- migrations, which run after dependent tenant data has been dropped.
DELETE FROM ln_plan_feature;
DELETE FROM ln_plan_limit;

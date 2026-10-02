CREATE TABLE ln_plan_limit (
    plan_id UUID NOT NULL,
    limit_type_id UUID NOT NULL,
    limit_value BIGINT NOT NULL,

    PRIMARY KEY (plan_id, limit_type_id),
    CONSTRAINT fk_ln_plan_limit_plan
        FOREIGN KEY (plan_id)
        REFERENCES md_subscription_plan(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_ln_plan_limit_type
        FOREIGN KEY (limit_type_id)
        REFERENCES di_limit_type(id),
    CONSTRAINT chk_ln_plan_limit_value
        CHECK (limit_value >= 0)
);

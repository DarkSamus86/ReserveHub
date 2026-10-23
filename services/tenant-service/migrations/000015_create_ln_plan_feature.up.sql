CREATE TABLE ln_plan_feature (
    plan_id UUID NOT NULL,
    feature_id UUID NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,

    PRIMARY KEY (plan_id, feature_id),
    CONSTRAINT fk_ln_plan_feature_plan
        FOREIGN KEY (plan_id)
        REFERENCES md_subscription_plan(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_ln_plan_feature_feature
        FOREIGN KEY (feature_id)
        REFERENCES di_feature(id)
        ON DELETE CASCADE
);

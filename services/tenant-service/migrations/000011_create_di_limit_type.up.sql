CREATE TABLE di_limit_type (
    id UUID PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    unit VARCHAR(32) NOT NULL
);

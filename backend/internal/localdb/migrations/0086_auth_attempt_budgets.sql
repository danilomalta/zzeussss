CREATE TABLE auth_attempt_budgets (
 budget_key VARCHAR(64) PRIMARY KEY,
 window_unix BIGINT NOT NULL,
 attempts INTEGER NOT NULL CHECK(attempts > 0)
);

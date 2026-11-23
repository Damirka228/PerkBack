CREATE TABLE cashback_reservations(
    order_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    period_start DATE NOT NULL,
    amount_kopecks BIGINT NOT NULL CHECK (amount_kopecks >= 0),
    status TEXT NOT NULL DEFAULT 'reserved' CHECK (status IN ('reserved','credited','canceled')),

    FOREIGN KEY (user_id, period_start)
        REFERENCES cashback_monthly_limits (user_id, period_start)
);
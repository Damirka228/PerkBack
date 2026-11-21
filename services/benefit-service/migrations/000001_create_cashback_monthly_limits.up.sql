CREATE TABLE cashback_monthly_limits(
    user_id TEXT NOT NULL,
    period_start DATE NOT NULL CHECK (EXTRACT(DAY FROM period_start) = 1),
    limit_kopecks BIGINT NOT NULL CHECK(limit_kopecks>=0) DEFAULT 100000,
    reserved_kopecks BIGINT NOT NULL CHECK(reserved_kopecks>=0) DEFAULT 0,
    credited_kopecks BIGINT NOT NULL CHECK(credited_kopecks>=0) DEFAULT 0,

    CHECK (reserved_kopecks + credited_kopecks <= limit_kopecks),

    PRIMARY KEY(user_id, period_start)
);
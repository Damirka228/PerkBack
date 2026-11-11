CREATE TABLE promotions (
    id UUID PRIMARY KEY,
    shop_id TEXT NOT NULL,
    category_id TEXT NOT NULL,
    min_purchase_amount_kopecks BIGINT NOT NULL CHECK (min_purchase_amount_kopecks >= 0),
    discount_kopecks BIGINT NOT NULL CHECK (discount_kopecks >= 0),
    rate_bps INTEGER NOT NULL CHECK (rate_bps >= 0 AND rate_bps <= 10000)
);
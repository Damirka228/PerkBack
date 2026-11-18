CREATE TABLE promotions_outbox (
    id UUID PRIMARY KEY,
    event_type TEXT NOT NULL,
    payload JSON NOT NULL,
    status TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'done')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
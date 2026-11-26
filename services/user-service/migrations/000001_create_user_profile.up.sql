CREATE TABLE user_profiles (
    user_id TEXT PRIMARY KEY,
    selected_category_ids TEXT[] NOT NULL DEFAULT '{}' CHECK (cardinality(selected_category_ids) <= 5)
);
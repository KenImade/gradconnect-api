ALTER TABLE employer
    ADD COLUMN IF NOT EXISTS avg_difficulty_rating numeric(3,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS avg_experience_rating numeric(3,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS review_count integer NOT NULL DEFAULT 0;
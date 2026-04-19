ALTER TABLE employer
    DROP COLUMN IF EXISTS avg_difficulty_rating,
    DROP COLUMN IF EXISTS avg_experience_rating,
    DROP COLUMN IF EXISTS review_count;
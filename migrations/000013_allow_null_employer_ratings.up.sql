ALTER TABLE employer
    ALTER COLUMN avg_difficulty_rating DROP NOT NULL,
    ALTER COLUMN avg_difficulty_rating DROP DEFAULT,
    ALTER COLUMN avg_experience_rating DROP NOT NULL,
    ALTER COLUMN avg_experience_rating DROP DEFAULT;

UPDATE employer
SET avg_difficulty_rating = NULL,
    avg_experience_rating = NULL
WHERE review_count = 0;
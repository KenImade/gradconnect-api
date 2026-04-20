UPDATE employer
SET avg_difficulty_rating = COALESCE(avg_difficulty_rating, 0),
    avg_experience_rating = COALESCE(avg_experience_rating, 0);

ALTER TABLE employer
    ALTER COLUMN avg_difficulty_rating SET DEFAULT 0,
    ALTER COLUMN avg_difficulty_rating SET NOT NULL,
    ALTER COLUMN avg_experience_rating SET DEFAULT 0,
    ALTER COLUMN avg_experience_rating SET NOT NULL;
DROP TRIGGER IF EXISTS application_track_updated_at ON application_track;
DROP TRIGGER IF EXISTS employer_updated_at ON employer;
DROP TRIGGER IF EXISTS assessment_profile_updated_at ON assessment_profile;
DROP TRIGGER IF EXISTS app_user_updated_at ON app_user;
DROP TRIGGER IF EXISTS review_updated_at ON review;
DROP TRIGGER IF EXISTS opportunity_updated_at ON opportunity;

DROP FUNCTION IF EXISTS set_updated_at();
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS application_track_updated_at ON application_track;
CREATE TRIGGER application_track_updated_at
    BEFORE UPDATE ON application_track
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS employer_updated_at ON employer;
CREATE TRIGGER employer_updated_at
    BEFORE UPDATE ON employer
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS assessment_profile_updated_at ON assessment_profile;
CREATE TRIGGER assessment_profile_updated_at
    BEFORE UPDATE ON assessment_profile
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS app_user_updated_at ON app_user;
CREATE TRIGGER app_user_updated_at
    BEFORE UPDATE ON app_user
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS review_updated_at ON review;
CREATE TRIGGER review_updated_at
    BEFORE UPDATE ON review
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS opportunity_updated_at ON opportunity;
CREATE TRIGGER opportunity_updated_at
    BEFORE UPDATE ON opportunity
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
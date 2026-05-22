CREATE OR REPLACE FUNCTION public.opportunity_search_vector_update()
    RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = public, pg_catalog
AS $function$
BEGIN
    NEW.search_vector :=
            setweight(to_tsvector('english', NEW.title), 'A') ||
            setweight(to_tsvector('english', (SELECT name FROM employer WHERE id = NEW.employer_id)), 'A') ||
            setweight(to_tsvector('english', NEW.location), 'B') ||
            setweight(to_tsvector('english', COALESCE(array_to_string(NEW.discipline_tags, ' '), '')), 'B') ||
            setweight(to_tsvector('english', (SELECT industry FROM employer WHERE id = NEW.employer_id)), 'B') ||
            setweight(to_tsvector('english', COALESCE(NEW.requirements, '')), 'C') ||
            setweight(to_tsvector('english', COALESCE(NEW.description, '')), 'C');
    RETURN NEW;
END;
$function$;
-- this is a function that emits a postgresql warning log
-- to test pgx/v5 tracelog logging
CREATE OR REPLACE FUNCTION test_warning_log()
RETURNS void AS $$
BEGIN
  RAISE WARNING 'This is a test warning message';
END;
$$ LANGUAGE plpgsql;

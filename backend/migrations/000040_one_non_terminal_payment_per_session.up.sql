DO $$
DECLARE
    duplicate_sessions TEXT;
BEGIN
    SELECT string_agg(
               format('%s (%s payments)', session_id, payment_count),
               ', ' ORDER BY session_id
           )
    INTO duplicate_sessions
    FROM (
        SELECT session_id, COUNT(*) AS payment_count
        FROM payments
        WHERE status IN ('pending', 'requested', 'provider_pending', 'requires_staff_confirmation')
        GROUP BY session_id
        HAVING COUNT(*) > 1
        LIMIT 20
    ) AS duplicates;

    IF duplicate_sessions IS NOT NULL THEN
        RAISE EXCEPTION
            'cannot enforce one non-terminal payment per session; resolve duplicates first: %',
            duplicate_sessions
            USING ERRCODE = '23505';
    END IF;
END $$;

CREATE UNIQUE INDEX idx_payments_one_non_terminal_per_session
    ON payments(session_id)
    WHERE status IN ('pending', 'requested', 'provider_pending', 'requires_staff_confirmation');

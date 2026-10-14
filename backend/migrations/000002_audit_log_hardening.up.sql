-- Audit hardening.
--
-- 1. Activity-log rows must survive the deletion of the participant they
--    describe: the FK switches from ON DELETE CASCADE to ON DELETE SET NULL.
-- 2. New action types for the events that were not audited so far.

ALTER TABLE activity_log
    DROP CONSTRAINT IF EXISTS activity_log_participant_id_fkey,
    ADD CONSTRAINT activity_log_participant_id_fkey
        FOREIGN KEY (participant_id) REFERENCES participant(id) ON DELETE SET NULL;

INSERT INTO action_type_code (code, name_en, name_fr) VALUES
    ('participant_deleted', 'Participant deleted', 'Participant supprimé'),
    ('data_exported',       'Data exported',       'Données exportées')
ON CONFLICT (code) DO UPDATE SET name_en = EXCLUDED.name_en, name_fr = EXCLUDED.name_fr;

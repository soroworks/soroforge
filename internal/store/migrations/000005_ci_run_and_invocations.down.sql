-- Revert migration 000005: drop columns fn_args, fn_name, and ci_run_url,
-- and restore the action constraint from migration 000004.
ALTER TABLE deployments
    DROP COLUMN fn_args,
    DROP COLUMN fn_name,
    DROP COLUMN ci_run_url;

ALTER TABLE deployments
    DROP CONSTRAINT deployments_action_check;

ALTER TABLE deployments
    ADD CONSTRAINT deployments_action_check CHECK (
        action IN ('deploy', 'upgrade', 'upload', 'extend', 'restore')
    );

-- Ledger of weekly digests already sent.
--
-- The primary key is the whole mechanism. Every API replica runs the same
-- schedule, so the send is claimed with an INSERT ... ON CONFLICT DO NOTHING:
-- exactly one replica gets the row and mails the org, and every other replica —
-- and every rerun of the same week — conflicts and does nothing. Without it,
-- scaling the deployment to two pods would mail every org twice.
--
-- week_start is a DATE rather than a timestamp so the key is the week itself,
-- not the instant a particular pod happened to evaluate it.
CREATE TABLE IF NOT EXISTS digest_sends (
    org_id     UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    week_start DATE NOT NULL,
    sent_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    recipients INT NOT NULL DEFAULT 0,
    PRIMARY KEY (org_id, week_start)
);

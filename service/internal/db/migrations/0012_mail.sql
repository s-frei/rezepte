-- Mail through an SMTP server the owner configures in the UI. Empty
-- smtp_host means "not configured here"; REZEPTE_SMTP_* beat all of it. The
-- password is plain text like link_preview_key: whoever reads this database
-- already holds everything mail could protect.
-- setup_links.sent_to is the address a link was mailed to, empty when it was
-- handed over by hand; redeeming a mailed link verifies that address.
-- +goose Up
ALTER TABLE instance_settings ADD COLUMN smtp_host TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_settings ADD COLUMN smtp_port INTEGER NOT NULL DEFAULT 587;
ALTER TABLE instance_settings ADD COLUMN smtp_security TEXT NOT NULL DEFAULT 'starttls';
ALTER TABLE instance_settings ADD COLUMN smtp_username TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_settings ADD COLUMN smtp_password TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_settings ADD COLUMN smtp_from TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_settings ADD COLUMN smtp_from_name TEXT NOT NULL DEFAULT 'Rezepte';
ALTER TABLE setup_links ADD COLUMN sent_to TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE setup_links DROP COLUMN sent_to;
ALTER TABLE instance_settings DROP COLUMN smtp_from_name;
ALTER TABLE instance_settings DROP COLUMN smtp_from;
ALTER TABLE instance_settings DROP COLUMN smtp_password;
ALTER TABLE instance_settings DROP COLUMN smtp_username;
ALTER TABLE instance_settings DROP COLUMN smtp_security;
ALTER TABLE instance_settings DROP COLUMN smtp_port;
ALTER TABLE instance_settings DROP COLUMN smtp_host;

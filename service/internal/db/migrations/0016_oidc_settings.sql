-- Sign-in through an OpenID Connect provider the owner configures in the UI.
-- Empty oidc_issuer means "not configured here"; REZEPTE_OIDC_* beat all of
-- it and never seed it. The client secret is plain text like smtp_password:
-- whoever reads this database already holds everything it could protect.
-- +goose Up
ALTER TABLE instance_settings ADD COLUMN oidc_issuer TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_settings ADD COLUMN oidc_client_id TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_settings ADD COLUMN oidc_client_secret TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_settings ADD COLUMN oidc_name TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE instance_settings DROP COLUMN oidc_name;
ALTER TABLE instance_settings DROP COLUMN oidc_client_secret;
ALTER TABLE instance_settings DROP COLUMN oidc_client_id;
ALTER TABLE instance_settings DROP COLUMN oidc_issuer;

-- Sharing a shopping list (docs/memory/content/features/shopping-lists.mdx):
-- the creator invites people; a row without accepted_at is a pending
-- invitation and grants nothing, an accepted row grants everything except
-- deleting the list. updated_by names who changed a list last; it is NULL
-- once that account is gone.
-- +goose Up
CREATE TABLE shopping_list_members (
    list_id     TEXT NOT NULL REFERENCES shopping_lists(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invited_at  TEXT NOT NULL,
    accepted_at TEXT,
    PRIMARY KEY (list_id, user_id)
);
CREATE INDEX shopping_list_members_user_idx ON shopping_list_members(user_id);
ALTER TABLE shopping_lists ADD COLUMN updated_by TEXT REFERENCES users(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE shopping_lists DROP COLUMN updated_by;
DROP TABLE shopping_list_members;

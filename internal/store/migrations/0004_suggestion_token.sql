-- The token that made a suggestion, so that an agent can withdraw its own proposals and no
-- one else's. NULL for suggestions made before this column existed: only the user can
-- remove those.
ALTER TABLE suggestions ADD COLUMN token_id TEXT;

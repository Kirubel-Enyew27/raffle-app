-- Deactivate first so the game disappears from the catalogue even when
-- historic matches reference it. Deleting the row is only possible when no
-- match would violate the p2p_matches -> p2p_games foreign key; match history
-- and its wallet ledger references are preserved rather than cascaded away.
UPDATE p2p_games SET is_active = FALSE WHERE slug = 'ethiopian-dama';

DELETE FROM p2p_games
WHERE slug = 'ethiopian-dama'
  AND NOT EXISTS (SELECT 1 FROM p2p_matches WHERE game_slug = 'ethiopian-dama');

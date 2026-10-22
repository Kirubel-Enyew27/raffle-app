DROP INDEX IF EXISTS idx_game_bets_user_created;
DROP INDEX IF EXISTS idx_game_bets_round_status;
DROP INDEX IF EXISTS idx_game_bets_round;
DROP TABLE IF EXISTS game_bets;

DROP INDEX IF EXISTS idx_game_rounds_settled;
DROP INDEX IF EXISTS idx_game_rounds_room_number;
DROP INDEX IF EXISTS uq_game_rounds_live_room;
DROP TABLE IF EXISTS game_rounds;

DROP SEQUENCE IF EXISTS crash_round_number_seq;

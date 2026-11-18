# Ethiopian Dama (Ethiopian Draughts)

Status: implemented as a fourth P2P game (`ethiopian-dama`) in the existing
`p2p` module. Free and staked play use the same authoritative engine; paid
matches are controlled by the `paid_matches_enabled` setting, which is enabled
by default.

## 1. Why this document exists

Ethiopian Dama (ዳማ) is a family of draughts/checkers games played in Ethiopia.
Published, authoritative rules are scarce, regional and barbershop rules vary,
and most written descriptions are second-hand. This document therefore records:

1. the exact default ruleset the server enforces,
2. which parts are reasonably well attested,
3. which parts are explicit implementation decisions, and
4. how the rule flags can be changed later without rewriting the game.

No rule below is presented as historically verified when it is not.

### Sources consulted

- `dama.games` (Ethiopian Dama online game) — describes the Ethiopian rule set
  as using **flying kings** ("kings that soar across the board"). This supports
  the flying-king default.
- English Wikipedia, *Checkers* — definitions of men, kings, flying kings,
  compulsory capture, multi-capture, promotion and blocked-player loss. Used
  for the shared draughts mechanics, not as an Ethiopia-specific source.
- Anecdotal player reports (Reddit r/Eritrea, Habesha players) describe local
  Dama as checkers with forward-only men and flying kings, similar to Italian
  checkers. This is weak evidence and is treated as such.
- No Ethiopian federation rulebook was located online. That absence is the main
  reason every contested rule is a configuration flag with a documented
  default rather than a hard-coded fact.

## 2. Default ruleset (authoritative)

| Rule | Default | Confidence |
| --- | --- | --- |
| Board | 8×8, only dark squares used (32 playable) | High (standard draughts board) |
| Pieces per side | 12 men | High |
| Initial placement | Light on rows 6–8 (bottom three rows), dark on rows 1–3, one man per dark square | High (standard 8×8 draughts setup) |
| First move | Light (player index 0, the room creator) | Implementation decision |
| Men movement | One square diagonally forward only (non-capturing moves) | Medium (local reports) |
| Men capture | One-square jump diagonally forward **or backward**; the jumped square must hold an enemy and the landing square must be empty and on the board | Implementation decision; configurable via `MenCaptureBackwards` (default on) |
| Kings | Flying kings: slide any distance diagonally; capture by jumping an enemy at any distance and landing on any empty square beyond it | Medium-high (`dama.games`, international-family rules) |
| Mandatory capture | Yes. If any capture exists, a non-capturing move is illegal | High for most draughts; implementation decision for Dama |
| Multi-capture | Yes. A capturing piece must continue jumping until no further capture is available | High |
| Capture choice | Any capture sequence may be chosen; maximum capture is **not** required | Implementation decision (common in casual/Ethiopian reports) |
| Promotion | A man that ends its move on the opponent's back rank becomes a king | High |
| Promotion during a jump chain | Not promoted until the chain ends; the man keeps its man capture directions (forward and backward) during the chain and only becomes a king on the last hop | Implementation decision |
| Backward capture by men | Allowed. Men capture forwards and backwards, including changing direction between jumps in one chain, while still moving forwards only when not capturing | Implementation decision; configurable |
| Draw | 50 moves per side without a capture or promotion (100 plies); threefold repetition of the same position | Implementation decision (standard draughts draw rules) |
| Blocked player | A player with no legal move loses | High |
| Resignation | Resigning forfeits the match; the opponent is awarded the result | Matches existing P2P policy |
| Turn timeout | 30 seconds per hop; timeout forfeits the match | Matches existing P2P timeout engine |
| Reconnect | State is fully reconstructed from the server board; no move is replayed client-side | Matches existing P2P policy |

The opening player is not randomized: the room creator is seated as Light and
moves first. This follows the existing P2P seating convention and keeps the
choice entirely server-side. Invite codes use the existing crypto/rand
generator; no new randomness is introduced by Dama.

### Regional / configurable variations

`backend/internal/dama/dama.go` exposes a `Ruleset` used by the engine. The
currently implemented flags are:

- `MenMoveBackwards` (default off: men may only *move* forwards)
- `MenCaptureBackwards` (default on: men may *capture* forwards and backwards;
  independent of `MenMoveBackwards`, so backward captures never grant a
  backwards ordinary step)
- `FlyingKings`
- `MandatoryCapture`
- `MajorityCapture`
- `PromotionDuringCapture`
- `HalfmoveDrawPlies`
- `RepetitionDraw`

Changing a default is a one-line change plus a migration of the `rules_json`
for the `p2p_games` row; no HTTP, UI or wallet code needs to change. Rules are
server-authoritative: the client receives rule metadata for display but never
sends rules.

### Explicitly not implemented

- Draw by agreement (players can resign; there is no agreed-draw endpoint).
- "Huffing" (removing a piece as punishment for declining a capture). Mandatory
  capture is enforced by rejecting the illegal move instead.
- Majority-capture tournaments (`MajorityCapture` exists but defaults off).
- Timed incremental clocks, increment per move, or tournament move limits.

## 3. Architecture

```
backend/internal/dama/dama.go              pure rules engine (no I/O, no HTTP, no wallet)
backend/internal/p2p/domain/dama.go        adapter implementing domain.RuleEngine
backend/internal/p2p/application/service.go
                                           authoritative match/turn/wallet/settlement flow
backend/internal/p2p/interfaces/http/*     existing match endpoints + stale-move error mapping
frontend/src/features/p2p/DamaBoard.tsx    board, selection, legal-move hints, capture cues
frontend/src/features/p2p/dama.ts          pure display/notation helpers (unit tested)
frontend/src/pages/app/P2PMatchPage.tsx    match lifecycle, history, resign, results
```

The rules engine knows nothing about users, HTTP, WebSockets, money or the
database. It operates on a `Board` and a `Game` value and is deterministic: the
same starting state plus the same sequence of `(from, to)` steps produces the
same board, history and result.

### Server-authoritative move flow

1. `POST /api/v1/p2p/matches/:id/move` with `{ "from": 52, "to": 43, "turn_seq": 7 }`.
2. The service opens a transaction and locks the match row (`FOR UPDATE`).
3. It checks status, membership, turn ownership and the turn deadline.
4. The Dama adapter validates the step against the **server** board, including
   `turn_seq` when supplied.
5. The move is applied to an in-memory copy; invalid moves return an error and
   the stored board is never mutated.
6. The move is appended to `p2p_match_moves` with the resulting board snapshot.
7. If the capture chain continues, the same player keeps the turn and the
   deadline is refreshed. Otherwise the turn switches.
8. Terminal states settle through the existing escrow/settlement service and
   publish the existing realtime events.

Two concurrent submissions cannot both advance the same turn: the row lock
serialises them, and the second request sees either a different
`current_turn_user_id` or a newer `turn_seq`.

### Board state (JSONB `p2p_matches.board_state`)

```jsonc
{
  "variant": "ethiopian-dama",
  "board_size": 8,
  "cells": [ /* 64 ints: 0 empty, 1 light man, 2 light king, 3 dark man, 4 dark king */ ],
  "turn": 0,
  "turn_seq": 12,
  "move_count": 12,
  "halfmove_clock": 3,
  "must_continue_from": null,
  "legal_moves": [
    { "from": 52, "to": 43, "captures": [], "promotion": false, "continues": false }
  ],
  "last_move": { "from": 52, "to": 43, "captures": [35], "promotion": false },
  "move_history": [ { "seq": 1, "player": 0, "from": 52, "to": 43, "captures": [35] } ],
  "repetitions": { "hash": 1 },
  "result": null
}
```

`legal_moves` contains **next hops**, not complete multi-jump sequences. That
lets the UI highlight one square at a time while the server still enforces the
whole chain. A step with `continues: true` means the same piece must keep
jumping; `must_continue_from` names that square.

### Anti-cheat properties

- The client never sends a board, winner, score, payout or result.
- Out-of-turn moves are rejected before the engine runs.
- Stale moves are rejected with `409 MATCH_CONFLICT` when `turn_seq` does not
  match the stored sequence.
- Duplicate submissions during a capture chain must use the current
  `must_continue_from` square and the current `turn_seq`.
- Match membership is checked on every state read and mutation.
- Settlement runs inside the same transaction as the final move and the
  existing `MarkCompletedTx` guard prevents a second settlement.
- Paid Dama matches are blocked by the same compliance switch as the other P2P
  games (`ErrPaidMatchesDisabled`).

## 4. APIs

No new endpoints are required. Dama reuses:

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/p2p/games` | Lists Dama alongside the other games |
| POST | `/p2p/matches` | Create/quick-match with `game_slug: "ethiopian-dama"` |
| POST | `/p2p/matches/:id/join` | Join a public room |
| POST | `/p2p/matches/join-code` | Join a private room |
| POST | `/p2p/matches/:id/ready` | Ready up; starts the match |
| POST | `/p2p/matches/:id/move` | Submit `{from, to, turn_seq}` |
| GET | `/p2p/matches/:id/state` | Reconnect / resync |
| POST | `/p2p/matches/:id/resign` | Forfeit |
| POST | `/p2p/matches/:id/rematch` | Request/accept a return match |
| GET | `/p2p/matches/my` | History, including Dama |
| GET | `/p2p/leaderboard?game=ethiopian-dama` | Game-filtered leaderboard |

Realtime events are the existing `p2p.*` events on channel
`p2p_match:<matchId>`: `p2p.move_made`, `p2p.match_started`,
`p2p.match_completed`, `p2p.turn_timeout`, `p2p.match_cancelled`,
`p2p.rematch_requested`, `p2p.rematch_created`. No new event names are needed.

Error mapping for Dama:

- illegal/forged step → `400 INVALID_ACTION`
- not your turn / match over / stale `turn_seq` / timeout → `409 MATCH_CONFLICT`
- non-participant → `403 FORBIDDEN`
- paid match while compliance switch is off → `403 PAID_MATCHES_DISABLED`

## 5. Database

Migration `20240101000016_add_ethiopian_dama.up.sql` inserts the
`p2p_games` row:

- slug `ethiopian-dama`
- 2 players, 30-second turn clock
- free-play compatible min stake (0) and the existing max stake
- `rules_json` documenting the chosen defaults

No table changes are needed: `board_state`, `scores`, `p2p_match_moves`, match
timestamps and player rows already persist everything required for history,
statistics, leaderboard and replay.

The 30-second clock is enforced by `turnSecondsFor` in
`internal/p2p/application/service.go` and mirrored by
`default_turn_seconds` in this seed. Changing one without the other would make
the catalogue metadata disagree with the enforced clock; update both together
until the service reads the clock from `p2p_games`.

`20240101000016_add_ethiopian_dama.down.sql` deactivates the row, then removes
it only when no match references it. Existing Dama matches (and their wallet
ledger references) are preserved, and re-running the up migration reactivates
the game.

## 6. Frontend

- Dama is listed automatically from `GET /p2p/games`; the lobby icon map has a
  Dama entry and the custom-match modal picks it up with no extra code.
- `DamaBoard` renders an 8×8 board oriented so the local player's pieces are at
  the bottom. Only dark squares are interactive.
- Selecting a piece highlights every server-provided legal destination.
  Captures are outlined in red, promotions in gold.
- During a capture chain the board auto-selects `must_continue_from` and shows
  a "continue capturing" banner; other pieces are not selectable.
- The last move is highlighted; captured squares flash briefly and a promoted
  piece gets a crown animation. Animations are disabled under
  `prefers-reduced-motion`.
- Every playable square is a real `<button>` with an accessible label, so
  keyboard and screen-reader users can select pieces and destinations.
- A move list shows algebraic notation (`e3–d4`, `×`, `=K`) from
  `board_state.move_history`.
- Waiting, ready, reconnecting, in-progress, won/lost/drawn and forfeited
  states reuse the existing match page. The SSE indicator doubles as the
  connection status. There is no per-player online presence in the current
  schema, so the UI shows connection health rather than a presence dot.
- Sound uses the existing Web Audio `SoundManager`: match start, your turn,
  timer ticks, move/capture placement, victory, defeat and draw. Muting is
  persisted by the existing `SoundToggle`; sound is never required to
  understand the state.

## 7. Tests

Backend:

- `backend/internal/dama/dama_test.go` — setup, legal/illegal movement,
  forward and backward captures by men, own-piece and illegal-landing
  rejection, backward non-capturing moves staying illegal, direction changes
  inside a capture chain, mandatory capture, multi-capture, capture choice,
  flying kings, promotion, blocked player, draw rules, state immutability and
  deterministic replay.
- `backend/internal/p2p/domain/dama_test.go` — board JSON round-trip, turn
  sequence enforcement, turn-continuation signalling and result mapping.
- `backend/internal/p2p/application/dama_integration_test.go` — full match over
  the service, out-of-turn rejection, stale/duplicate rejection, a backward
  capture that both players observe identically, reconnect state, resignation,
  concurrent submissions, a staked win that verifies the reservation,
  configured fee, net payout and duplicate-settlement guard, and a staked draw
  that verifies both stakes are refunded with no fee (skips when Postgres is
  unavailable, like the existing integration suite).

Frontend:

- `frontend/src/features/p2p/dama.test.ts` — square/orientation mapping,
  legal-destination lookup, notation, capture-chain display rules.

Existing checks continue to run: `go build ./...`, `go vet ./...`,
`go test ./...`, `npm test`, `npm run typecheck:test`, `npm run lint`,
`npm run build`.

## 8. Operations

Run migrations as usual:

```bash
cd backend
make migrate-up        # or the project's goose/migrate command
```

If a development database is already migrated, apply
`20240101000016_add_ethiopian_dama.up.sql` manually or rerun the migration
tool. Until the row exists, `CreateMatch` returns 404 for `ethiopian-dama`.

No new environment variables are required. No wallet configuration changes are
required. Dama supports both competition modes; staked matches use the same
wallet reservation, stake validation and settlement path as every other P2P
game.

## 9. Known limitations

- The default ruleset is a documented implementation choice, not a verified
  national rulebook. Regional variations are configurable but not exposed in
  the UI.
- Draw by agreement and tournament clocks are not implemented.
- The current schema has no persistent online-presence flag; the UI reports SSE
  connection health rather than "opponent online".
- Capture chains are submitted one hop at a time. This is deliberate (it gives
  clear UI feedback and precise timeout accounting) but differs from clients
  that expect a single long move payload.
- Payloads from older clients that omit `turn_seq` are still accepted for
  backwards compatibility; the in-progress board still prevents duplicate
  moves because the turn/chain state is authoritative.

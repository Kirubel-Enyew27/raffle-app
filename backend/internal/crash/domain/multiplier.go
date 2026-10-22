package domain

import (
	"math"
	"strings"
	"time"
)

// MinMultiplierBP is the floor: a round never crashes below 1.00x.
const MinMultiplierBP = 100

// MultiplierAt is the authoritative multiplier after elapsed seconds of a round.
//
// This is timing/display math — an exponential curve — not money math. Payouts
// are always computed from the returned basis-point integer with exact integer
// arithmetic (see pkg/money), so floating point never touches a balance.
func MultiplierAt(elapsedSeconds float64, growthRate float64) int {
	if elapsedSeconds <= 0 {
		return MinMultiplierBP
	}
	bp := int(math.Floor(math.Exp(growthRate*elapsedSeconds) * 100))
	if bp < MinMultiplierBP {
		bp = MinMultiplierBP
	}
	return bp
}

// DurationForMultiplier inverts MultiplierAt: how many seconds a round runs
// before the multiplier reaches multiplierBP.
func DurationForMultiplier(multiplierBP int, growthRate float64) float64 {
	if multiplierBP <= MinMultiplierBP || growthRate <= 0 {
		return 0
	}
	return math.Log(float64(multiplierBP)/100) / growthRate
}

// StorageTolerance absorbs the granularity mismatch between the local clock and
// PostgreSQL.
//
// timestamptz is stored at microsecond resolution and is rounded on write, so a
// timestamp we recorded can read back up to half a microsecond *ahead* of the
// local clock. Comparing the two at nanosecond granularity would then make an
// already-elapsed deadline look like it is still in the future. Every deadline
// comparison therefore allows this tolerance.
//
// It is safe in both directions here: for the crash instant it makes the engine
// and cash-out marginally *stricter* (never later than correct), and for the
// post-crash display windows it can only advance settlement by under a
// millisecond. Settlement is idempotent, so that cannot affect money.
const StorageTolerance = time.Millisecond

// CrashAt returns the instant a round crashes, derived from its start time and
// the seed-committed crash point.
func CrashAt(startedAt time.Time, crashBP int, growthRate float64) time.Time {
	seconds := DurationForMultiplier(crashBP, growthRate)
	return startedAt.Add(time.Duration(seconds * float64(time.Second)))
}

// MultiplierValue converts basis points to the player-facing multiplier
// (247 -> 2.47). Used for JSON responses only.
func MultiplierValue(multiplierBP int) float64 {
	return float64(multiplierBP) / 100
}

// MultiplierToBasisPoints converts a player-supplied multiplier into basis
// points, truncated to two decimals. Returns 0 when the input is not a usable
// multiplier so callers can reject it.
func MultiplierToBasisPoints(value float64) int {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 1 {
		return 0
	}
	// Round to two decimals before converting so 1.005x does not silently
	// become 1.00x or 1.01x depending on binary representation.
	scaled := math.Round(value*100) / 100
	return int(math.Round(scaled * 100))
}

// DisplayName produces the public name shown in the live multiplayer table.
//
// A full name is used as-is (it is already public inside the app). Without one,
// the email is reduced to a masked form such as "ku***@example.com" so a player
// can recognise their own row without leaking another player's address.
func DisplayName(fullName, email string) string {
	if name := strings.TrimSpace(fullName); name != "" {
		return name
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return "Player"
	}
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return maskTail(email)
	}
	local, domain := email[:at], email[at:]
	return maskTail(local) + domain
}

// maskTail keeps at most the first two characters and masks the rest, cutting
// on rune boundaries so a multi-byte character is never split in half.
func maskTail(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return "***"
	}
	if len(runes) <= 2 {
		return string(runes[0]) + "***"
	}
	return string(runes[:2]) + "***"
}

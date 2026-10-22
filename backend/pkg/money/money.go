// Package money provides exact monetary arithmetic in minor units (cents).
//
// The rest of the application stores balances as DECIMAL(15,2) and moves them
// around as float64. Game math (stake debits, multiplier payouts) must never be
// computed in floating point, so the crash game performs every calculation with
// Cents (int64 minor units) and only converts to float64 at the wallet boundary.
package money

import (
	"fmt"
	"math"
)

// Cents is an exact amount of money in minor units (1 * Cents == 0.01 ETB).
type Cents int64

// Scale is the number of minor units in one major unit.
const Scale = 100

// MaxCents bounds the value we accept so that stake * multiplier cannot overflow.
// 10^12 cents == 10 billion ETB, far above any realistic stake or payout.
const MaxCents = Cents(1_000_000_000_000)

// FromFloat converts a major-unit amount to Cents, rounding half away from zero.
// This is the only place rounding of externally supplied amounts happens.
func FromFloat(amount float64) Cents {
	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0
	}
	return Cents(math.Round(amount * Scale))
}

// Float converts back to a major-unit float64. Used only at the wallet boundary
// (the existing wallet API is float64-based).
func (c Cents) Float() float64 {
	return float64(c) / Scale
}

// String renders the amount with exactly two decimal places.
func (c Cents) String() string {
	neg := c < 0
	v := c
	if neg {
		v = -v
	}
	s := fmt.Sprintf("%d.%02d", int64(v)/Scale, int64(v)%Scale)
	if neg {
		return "-" + s
	}
	return s
}

// Valid reports whether the amount is positive and within the safe range.
func (c Cents) Valid() bool {
	return c > 0 && c <= MaxCents
}

// MulByBasisPoints multiplies by a multiplier expressed in basis points
// (100 bp == 1.00x) and floors the result to whole cents.
//
// Flooring is deliberate: it never creates money, and any sub-cent remainder
// favours the house by less than one cent per payout.
func (c Cents) MulByBasisPoints(bp int) Cents {
	if bp <= 0 {
		return 0
	}
	product := c * Cents(bp)
	return product / 100
}

// Add returns the sum, guarding against overflow-related nonsense by refusing
// values beyond MaxCents.
func (c Cents) Add(other Cents) Cents {
	sum := c + other
	if sum > MaxCents {
		return MaxCents
	}
	return sum
}

// RoundToCents normalises arbitrary float64 arithmetic (for example a value
// produced by an external verifier) to an exact two-decimal amount.
func RoundToCents(amount float64) float64 {
	return FromFloat(amount).Float()
}

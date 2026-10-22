package money

import (
	"math"
	"testing"
)

func TestFromFloatRoundsToCents(t *testing.T) {
	cases := []struct {
		in   float64
		want Cents
	}{
		{0, 0},
		{1, 100},
		{12.34, 1234},
		{0.005, 1}, // rounds half away from zero
		{0.004, 0}, // below half a cent
		{10.999, 1100},
		{33.33, 3333},
		{-4.25, -425},
		{math.NaN(), 0},
		{math.Inf(1), 0},
	}
	for _, c := range cases {
		if got := FromFloat(c.in); got != c.want {
			t.Errorf("FromFloat(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestFloatRoundTripIsStable(t *testing.T) {
	for _, v := range []float64{0.01, 9.99, 100, 1234.56, 999999.99} {
		if got := FromFloat(v).Float(); math.Abs(got-v) > 1e-9 {
			t.Errorf("round trip of %v produced %v", v, got)
		}
	}
}

func TestMulByBasisPointsFloors(t *testing.T) {
	cases := []struct {
		stake Cents
		bp    int
		want  Cents
	}{
		{10000, 100, 10000}, // 1.00x returns the stake
		{10000, 130, 13000}, // 1.30x
		{10000, 247, 24700}, // 2.47x
		{3333, 107, 3566},   // 3566.31 floors to 3566, never rounds up
		{1, 999, 9},         // 0.01 at 9.99x
		{10000, 0, 0},       // invalid multiplier pays nothing
		{10000, -5, 0},      // negative multiplier pays nothing
	}
	for _, c := range cases {
		if got := c.stake.MulByBasisPoints(c.bp); got != c.want {
			t.Errorf("%d cents at %d bp = %d, want %d", c.stake, c.bp, got, c.want)
		}
	}
}

func TestMulNeverCreatesRoundingMoney(t *testing.T) {
	// Flooring must never pay more than the exact product.
	for _, stake := range []Cents{1, 7, 99, 12345} {
		for bp := 100; bp <= 100000; bp += 137 {
			exact := float64(stake) * float64(bp) / 100
			if got := float64(stake.MulByBasisPoints(bp)); got > exact {
				t.Fatalf("stake %d at %d bp paid %.4f > exact %.4f", stake, bp, got, exact)
			}
		}
	}
}

func TestValidBounds(t *testing.T) {
	if (Cents(0)).Valid() {
		t.Error("zero should not be a valid stake")
	}
	if (Cents(-1)).Valid() {
		t.Error("negative should not be a valid stake")
	}
	if !(Cents(1000)).Valid() {
		t.Error("10.00 should be a valid stake")
	}
	if (MaxCents + 1).Valid() {
		t.Error("values above MaxCents should be rejected")
	}
}

func TestAddClampsAtMax(t *testing.T) {
	if got := MaxCents.Add(MaxCents); got != MaxCents {
		t.Errorf("overflowing add should clamp to MaxCents, got %d", got)
	}
	if got := Cents(100).Add(Cents(250)); got != 350 {
		t.Errorf("100 + 250 = %d, want 350", got)
	}
}

func TestString(t *testing.T) {
	cases := map[Cents]string{
		0:    "0.00",
		5:    "0.05",
		100:  "1.00",
		1234: "12.34",
		-425: "-4.25",
	}
	for in, want := range cases {
		if got := in.String(); got != want {
			t.Errorf("%d renders %q, want %q", in, got, want)
		}
	}
}

func TestRoundToCents(t *testing.T) {
	if got := RoundToCents(12.3456); got != 12.35 {
		t.Errorf("RoundToCents(12.3456) = %v, want 12.35", got)
	}
	if got := RoundToCents(1.004); got != 1.0 {
		t.Errorf("RoundToCents(1.004) = %v, want 1", got)
	}
}

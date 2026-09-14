package world

import (
	"math"
	"testing"
)

func TestCreatureContactDistanceMatchesReferenceMeleeRange(t *testing.T) {
	if got := calcMeleeRange(1.5, 1.5); got != 5 {
		t.Fatalf("default contact distance=%v want=5", got)
	}
	if got := calcMeleeRange(4, 3); math.Abs(got-(7+4.0/3.0)) > 0.00001 {
		t.Fatalf("large creature contact distance=%v want=%v", got, 7+4.0/3.0)
	}
}

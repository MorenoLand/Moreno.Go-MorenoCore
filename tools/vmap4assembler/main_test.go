package main

import "testing"

func TestMin(t *testing.T) {
	if min(3, 5) != 3 {
		t.Errorf("min(3, 5) = %d, want 3", min(3, 5))
	}
	if min(10, 2) != 2 {
		t.Errorf("min(10, 2) = %d, want 2", min(10, 2))
	}
	if min(4, 4) != 4 {
		t.Errorf("min(4, 4) = %d, want 4", min(4, 4))
	}
}

func TestPrintBanner(t *testing.T) {
	printBanner()
}

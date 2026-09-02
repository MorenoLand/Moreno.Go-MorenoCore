package crypto

import (
	"encoding/hex"
	"testing"
	"time"
)

func TestTOTPReferenceVector(t *testing.T) {
	secret, err := hex.DecodeString("3132333435363738393031323334353637383930")
	if err != nil {
		t.Fatal(err)
	}
	if got := GenerateTOTP(secret, time.Unix(59, 0).UTC()); got != 287082 {
		t.Fatalf("token=%d", got)
	}
	if !ValidateTOTP(secret, 287082, time.Unix(59, 0).UTC()) {
		t.Fatal("token rejected")
	}
}


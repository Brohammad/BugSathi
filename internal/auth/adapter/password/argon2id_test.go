package password

import "testing"

func TestArgon2idRoundTrip(t *testing.T) {
	h := NewArgon2id()
	hash, err := h.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Compare(hash, "correct horse battery"); err != nil {
		t.Fatalf("compare ok: %v", err)
	}
	if err := h.Compare(hash, "wrong"); err == nil {
		t.Fatal("expected mismatch")
	}
}

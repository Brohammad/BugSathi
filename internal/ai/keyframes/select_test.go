package keyframes_test

import (
	"testing"

	"github.com/Brohammad/BugSathi/internal/ai/keyframes"
)

func TestSelectEvenSpacing(t *testing.T) {
	keys := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	got := keyframes.Select(keys, 5)
	want := []string{"a", "c", "e", "g", "j"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestSelectPassthrough(t *testing.T) {
	keys := []string{"a", "b"}
	got := keyframes.Select(keys, 5)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("%v", got)
	}
}

func TestSelectEmpty(t *testing.T) {
	if keyframes.Select(nil, 3) != nil {
		t.Fatal("expected nil")
	}
}

package shortener

import (
	"strings"
	"testing"
)

func TestGenerateCode_Length(t *testing.T) {
	for _, n := range []int{1, 7, 16} {
		code, err := GenerateCode(n)
		if err != nil {
			t.Fatalf("GenerateCode(%d): %v", n, err)
		}
		if len(code) != n {
			t.Errorf("got len = %d, want %d", len(code), n)
		}
	}
}

func TestGenerateCode_AlphabetOnly(t *testing.T) {
	code, err := GenerateCode(100)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range code {
		if !strings.ContainsRune(alphabet, c) {
			t.Errorf("invalid char %q in code", c)
		}
	}
}

func TestGenerateCode_NoCollisionsAtScale(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		code, err := GenerateCode(7)
		if err != nil {
			t.Fatal(err)
		}
		if seen[code] {
			t.Fatalf("duplicate code %q", code)
		}
		seen[code] = true
	}
}

package orders

import "testing"

func TestNormalizePhone(t *testing.T) {
	ok := map[string]string{
		"08030000000":       "0803 000 0000",
		"0803 000 0000":     "0803 000 0000",
		"+234 803 000 0000": "0803 000 0000",
		"2348030000000":     "0803 000 0000",
		"0912-345-6789":     "0912 345 6789",
		"07012345678":       "0701 234 5678",
	}
	for in, want := range ok {
		if got, valid := NormalizePhone(in); !valid || got != want {
			t.Errorf("NormalizePhone(%q) = %q, %v; want %q", in, got, valid, want)
		}
	}
	for _, bad := range []string{"", "0803000000", "080300000000", "06030000000", "12345678901", "+44 7700 900000"} {
		if _, valid := NormalizePhone(bad); valid {
			t.Errorf("NormalizePhone(%q) accepted, want rejected", bad)
		}
	}
}

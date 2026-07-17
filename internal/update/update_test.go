package update

import "testing"

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"2.0.0", "1.9.9", 1},
		{"1.10.0", "1.9.0", 1},
	}
	for _, tc := range cases {
		if got := compare(tc.a, tc.b); got != tc.want {
			t.Fatalf("compare(%q,%q)=%d want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestNormalize(t *testing.T) {
	if normalize("v1.2.3") != "1.2.3" {
		t.Fatal(normalize("v1.2.3"))
	}
	if normalize("dev") != "0.0.0" {
		t.Fatal(normalize("dev"))
	}
	if normalize("1.2.3-beta") != "1.2.3" {
		t.Fatal(normalize("1.2.3-beta"))
	}
}

package updater

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		left     string
		right    string
		expected int
	}{
		{left: "v1.0.0", right: "1.0.0", expected: 0},
		{left: "1.1.0", right: "1.0.9", expected: 1},
		{left: "1.0.9", right: "1.1.0", expected: -1},
		{left: "2.0", right: "2.0.0", expected: 0},
	}
	for _, test := range tests {
		if actual := CompareVersions(test.left, test.right); actual != test.expected {
			t.Fatalf("CompareVersions(%q, %q) = %d, want %d", test.left, test.right, actual, test.expected)
		}
	}
}

package game

import "testing"

func TestShouldShowMidBoard(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		n    int
		want map[int]bool
	}{
		{name: "eight", n: 8, want: map[int]bool{4: true}},
		{name: "ten", n: 10, want: map[int]bool{5: true}},
		{name: "sixteen", n: 16, want: map[int]bool{4: true, 8: true, 12: true}},
		{name: "fifty", n: 50, want: map[int]bool{10: true, 20: true, 30: true, 40: true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for q := 1; q <= tc.n; q++ {
				got := ShouldShowMidBoard(q, tc.n)
				if got != tc.want[q] {
					t.Fatalf("q=%d n=%d got %v want %v", q, tc.n, got, tc.want[q])
				}
			}
		})
	}
}

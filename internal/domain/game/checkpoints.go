package game

// ShouldShowMidBoard reports whether a mid-game scoreboard should appear
// after the given 1-based question number (not after the final question).
//
//	N ≤ 10  → once at floor(N/2)
//	11–20   → three times at N/4, N/2, 3N/4
//	N ≥ 21  → every 10th question
func ShouldShowMidBoard(questionNumber, total int) bool {
	if total <= 1 || questionNumber <= 0 || questionNumber >= total {
		return false
	}

	switch {
	case total <= 10:
		return questionNumber == total/2
	case total <= 20:
		for _, point := range midPointsThree(total) {
			if point == questionNumber {
				return true
			}
		}
		return false
	default:
		return questionNumber%10 == 0
	}
}

func midPointsThree(total int) []int {
	raw := []int{total / 4, total / 2, (3 * total) / 4}
	out := make([]int, 0, 3)
	seen := map[int]struct{}{}
	for _, point := range raw {
		if point <= 0 || point >= total {
			continue
		}
		if _, ok := seen[point]; ok {
			continue
		}
		seen[point] = struct{}{}
		out = append(out, point)
	}
	return out
}

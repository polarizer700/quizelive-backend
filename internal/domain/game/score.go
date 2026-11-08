package game

import "time"

// Points делит очки вопроса пополам: половина за верный ответ, половина за оставшееся время.
func Points(points int, correct bool, elapsed, limit time.Duration) int {
	if !correct || points < 1 || limit <= 0 {
		return 0
	}

	if elapsed < 0 {
		elapsed = 0
	}

	if elapsed > limit {
		elapsed = limit
	}

	remain := limit - elapsed
	bonus := int(remain * time.Duration(points) / 2 / limit)
	total := points/2 + bonus

	if total < 1 {
		return 1
	}

	if total > points {
		return points
	}

	return total
}

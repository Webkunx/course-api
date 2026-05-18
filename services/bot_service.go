package services

import (
	"time"

	"course-api/repository"
)

// BotService detects bot behaviour by querying dwell-time signals from the DB
// and applying threshold rules in a single integrated step.
type BotService struct {
	minSingleDwellMs int64
	minAvgDwellMs    int64
	maxDailyLessons  int64
	dwellWindow      int
}

func NewBotService(minSingle, minAvg, maxDaily int64, dwellWindow int) *BotService {
	return &BotService{
		minSingleDwellMs: minSingle,
		minAvgDwellMs:    minAvg,
		maxDailyLessons:  maxDaily,
		dwellWindow:      dwellWindow,
	}
}

// DetectNewStatus fetches dwell stats and daily count within tx, then returns the new status.
// Short-circuits immediately when the user is already flagged — saves two DB round-trips.
func (bs *BotService) DetectNewStatus(tx repository.TxRepo, userID string, slot int64, currentStatus string) (string, error) {
	if currentStatus == "bot" {
		return "bot", nil
	}

	stats, err := tx.GetDwellStats(userID, slot, bs.dwellWindow)
	if err != nil {
		return "", err
	}

	today := time.Now().UTC().Format("2006-01-02")
	daily, err := tx.IncrementDaily(userID, today)
	if err != nil {
		return "", err
	}

	if stats.SingleMs < bs.minSingleDwellMs {
		return "bot", nil
	}
	if stats.HasAvg && stats.AvgMs < float64(bs.minAvgDwellMs) {
		return "bot", nil
	}
	if daily > bs.maxDailyLessons {
		return "bot", nil
	}
	return "real", nil
}

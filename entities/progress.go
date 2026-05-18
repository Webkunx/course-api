package entities

type UserProgress struct {
	Status   string
	Cursor   int64
	IsActive bool
}

type DwellStats struct {
	SingleMs int64
	AvgMs    float64
	HasAvg   bool
}

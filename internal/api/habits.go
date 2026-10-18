package api

// Habit is a versioned rule; edits never rewrite prior calendar dates.
type Habit struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	Notes           string  `json:"notes"`
	ProjectID       *string `json:"project_id"`
	Weekdays        []int   `json:"weekdays"`
	Time            string  `json:"time"`
	DurationMinutes int     `json:"duration_minutes"`
	DayOffset       int     `json:"day_offset"`
	ReminderMinutes *int    `json:"reminder_minutes"`
	Paused          bool    `json:"paused"`
	Timezone        string  `json:"timezone"`
	EffectiveDate   string  `json:"effective_date"`
	Version         int     `json:"version"`
}

type HabitOccurrence struct {
	PlanDate        string  `json:"plan_date"`
	HabitID         string  `json:"habit_id"`
	Date            string  `json:"date"`
	Title           string  `json:"title"`
	Notes           string  `json:"notes"`
	ProjectID       *string `json:"project_id"`
	Timezone        string  `json:"timezone"`
	Time            string  `json:"time"`
	DayOffset       int     `json:"day_offset"`
	DurationMinutes int     `json:"duration_minutes"`
	ScheduledAt     *string `json:"scheduled_at"`
	EndAt           *string `json:"end_at"`
	RemindAt        *string `json:"remind_at"`
	Status          string  `json:"status"`
	CompletedAt     *string `json:"completed_at"`
	SkipReason      string  `json:"skip_reason"`
	Actor           Actor   `json:"actor"`
}

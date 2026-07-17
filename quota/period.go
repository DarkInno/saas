package quota

// Period describes quota reset cadence.
type Period string

const (
	PeriodNone  Period = "none"
	PeriodDay   Period = "day"
	PeriodMonth Period = "month"
)

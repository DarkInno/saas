package plan

// QuotaPeriod describes the reset period of a quota.
type QuotaPeriod string

const (
	QuotaPeriodNone  QuotaPeriod = "none"
	QuotaPeriodDay   QuotaPeriod = "day"
	QuotaPeriodMonth QuotaPeriod = "month"
)

// Quota describes a plan resource limit.
type Quota struct {
	Resource string
	Limit    int64
	Period   QuotaPeriod
}

package subscription

// Status describes subscription lifecycle state.
type Status string

const (
	StatusActive    Status = "active"
	StatusExpired   Status = "expired"
	StatusCancelled Status = "cancelled"
)

func validStatus(status Status) bool {
	switch status {
	case StatusActive, StatusExpired, StatusCancelled:
		return true
	default:
		return false
	}
}

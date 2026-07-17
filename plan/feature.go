package plan

// Feature describes a plan feature flag.
type Feature struct {
	Key     string
	Enabled bool
	Config  map[string]string
}

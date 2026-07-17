package feature

// Flag describes a feature flag.
type Flag struct {
	Key     string
	Enabled bool
	Config  map[string]string
}

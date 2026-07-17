package plan

// Plan describes SaaS package capabilities.
type Plan struct {
	ID       string
	Name     string
	Features []Feature
	Quotas   []Quota
}

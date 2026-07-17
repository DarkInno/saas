package tenantctx

// stateKey is a package-private distinct context key. It is deliberately not
// a string or exported type, so values set by other packages cannot collide
// with tenant context state.
type stateKey struct{}

type side uint8

const (
	sideNone side = iota
	sideTenant
	sideHost
)

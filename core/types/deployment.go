package types

// DeploymentUnitID identifies a host-managed logical deployment unit.
type DeploymentUnitID string

// String returns the string form of the deployment unit identifier.
func (id DeploymentUnitID) String() string {
	return string(id)
}

// DeploymentUnitStatus describes whether a deployment unit can accept tenant traffic.
type DeploymentUnitStatus string

const (
	// DeploymentUnitStatusActive accepts tenant traffic and new assignments.
	DeploymentUnitStatusActive DeploymentUnitStatus = "active"

	// DeploymentUnitStatusDisabled does not accept tenant traffic or new assignments.
	DeploymentUnitStatusDisabled DeploymentUnitStatus = "disabled"
)

// DeploymentUnit describes a host-managed logical location where tenant data and
// traffic may be placed. Infrastructure endpoints and credentials remain host-owned.
type DeploymentUnit struct {
	ID            DeploymentUnitID
	Status        DeploymentUnitStatus
	Region        string
	ResidencyTags []string
	Metadata      map[string]string
}

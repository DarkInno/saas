package types

import "testing"

func TestDeploymentUnitIDString(t *testing.T) {
	id := DeploymentUnitID("eu-central-1")
	if got := id.String(); got != "eu-central-1" {
		t.Fatalf("DeploymentUnitID.String() = %q, want %q", got, "eu-central-1")
	}
}

func TestDeploymentUnitStatusValues(t *testing.T) {
	statuses := []DeploymentUnitStatus{
		DeploymentUnitStatusActive,
		DeploymentUnitStatusDisabled,
	}

	for _, status := range statuses {
		if status == "" {
			t.Fatal("deployment unit status must not be empty")
		}
	}
}

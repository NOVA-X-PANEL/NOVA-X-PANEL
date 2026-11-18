package controller

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestForceCreateOwnershipBindsNonOwnerToSelf(t *testing.T) {
	owner, admin, _ := ownerGuidRegressionDB(t)

	ownerID := int64(99)
	forceCreateOwnership(admin, &ownerID)
	if ownerID != int64(admin.Id) {
		t.Fatalf("non-owner supplied owner id was preserved: got %d, want %d", ownerID, admin.Id)
	}

	ownerID = int64(admin.Id)
	forceCreateOwnership(owner, &ownerID)
	if ownerID != int64(admin.Id) {
		t.Fatalf("owner assignment was unexpectedly overwritten: got %d, want %d", ownerID, admin.Id)
	}

	legacy := &model.User{Id: admin.Id, RoleId: 0}
	legacyID := int64(0)
	forceCreateOwnership(legacy, &legacyID)
	if legacyID != int64(admin.Id) {
		t.Fatalf("account with unresolved/non-owner role was not safely bound: got %d, want %d", legacyID, admin.Id)
	}
}

package models

import "testing"

func TestRoleAndStatusTypesRejectUnknownValues(t *testing.T) {
	if !RoleAdmin.Valid() || !RoleStaff.Valid() || UserRole("owner").Valid() {
		t.Fatal("role validation accepted an invalid role or rejected a valid role")
	}
	for _, status := range []AccountStatus{AccountPending, AccountActive, AccountDisabled, AccountDeleted} {
		if !status.Valid() {
			t.Errorf("valid status rejected: %q", status)
		}
	}
	if AccountStatus("enabled").Valid() {
		t.Fatal("unknown account status accepted")
	}
	if !CodeSetup.Valid() || !CodeRecovery.Valid() || ActionCodePurpose("login").Valid() {
		t.Fatal("action code purpose validation failed")
	}
}

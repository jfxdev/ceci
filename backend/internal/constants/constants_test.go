package constants

import "testing"

func TestProjectRole_Meets(t *testing.T) {
	cases := []struct {
		role  ProjectRole
		min   ProjectRole
		meets bool
	}{
		{RoleOwner, RoleViewer, true},
		{RoleOwner, RoleOwner, true},
		{RoleViewer, RoleAdmin, false},
		{RoleAdmin, RoleEditor, true},
		{RoleEditor, RoleAdmin, false},
		{RoleViewer, RoleViewer, true},
	}
	for _, c := range cases {
		if got := c.role.Meets(c.min); got != c.meets {
			t.Errorf("%s.Meets(%s) = %v, want %v", c.role, c.min, got, c.meets)
		}
	}
}

func TestProjectRole_Valid(t *testing.T) {
	valid := []ProjectRole{RoleOwner, RoleAdmin, RoleEditor, RoleViewer}
	for _, r := range valid {
		if !r.Valid() {
			t.Errorf("%s should be valid", r)
		}
	}
	if ProjectRole("superadmin").Valid() {
		t.Error("unknown role should not be valid")
	}
}

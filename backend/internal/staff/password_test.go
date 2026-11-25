package staff

import "testing"

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := CheckPassword("correct horse battery", hash); err != nil || !ok {
		t.Fatalf("right password: ok=%v err=%v", ok, err)
	}
	if ok, _ := CheckPassword("wrong password", hash); ok {
		t.Fatal("wrong password accepted")
	}
	other, _ := HashPassword("correct horse battery")
	if other == hash {
		t.Fatal("two hashes of the same password are identical; the salt isn't random")
	}
	if _, err := CheckPassword("x", "not-a-hash"); err == nil {
		t.Fatal("malformed hash should return an error")
	}
}

func TestRolePermissions(t *testing.T) {
	tests := []struct {
		role Role
		perm Permission
		want bool
	}{
		{RoleOwner, PermStaff, true},
		{RoleManager, PermStaff, false},
		{RoleManager, PermCatalogue, true},
		{RoleContentEditor, PermProducts, true},
		{RoleContentEditor, PermCatalogue, false},
		{RolePacker, PermProducts, false},
		{RolePacker, PermOrders, true},
	}
	for _, tt := range tests {
		if got := tt.role.Can(tt.perm); got != tt.want {
			t.Errorf("%s can %s = %v, want %v", tt.role, tt.perm, got, tt.want)
		}
	}
}

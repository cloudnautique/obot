package poweruserworkspace

import (
	"testing"

	"github.com/obot-platform/obot/apiclient/types"
)

// TestAdminReadonlyKeepsWorkspace checks that a power user who is also granted Admin Read-Only keeps their workspace
// and what they published in it.
func TestAdminReadonlyKeepsWorkspace(t *testing.T) {
	tests := []struct {
		name           string
		role           types.Role
		wantWorkspace  bool
		wantPublishing bool
	}{
		{
			name:           "admin read-only alone",
			role:           types.RoleAdminReadonly,
			wantWorkspace:  false,
			wantPublishing: false,
		},
		{
			name:           "admin read-only and power user",
			role:           types.RoleAdminReadonly | types.RolePowerUser,
			wantWorkspace:  true,
			wantPublishing: false,
		},
		{
			name:           "admin read-only and power user plus",
			role:           types.RoleAdminReadonly | types.RolePowerUserPlus,
			wantWorkspace:  true,
			wantPublishing: true,
		},
		{
			name:           "admin read-only, power user plus and auditor",
			role:           types.RoleAdminReadonly | types.RolePowerUserPlus | types.RoleAuditor,
			wantWorkspace:  true,
			wantPublishing: true,
		},
	}

	h := &Handler{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := h.isPrivilegedRole(tt.role); got != tt.wantWorkspace {
				t.Errorf("isPrivilegedRole(%d) = %v, want %v", tt.role, got, tt.wantWorkspace)
			}
			// reconcileWorkspaceRole removes what a workspace published once its role loses Power User Plus.
			if got := tt.role.HasRole(types.RolePowerUserPlus); got != tt.wantPublishing {
				t.Errorf("Role(%d).HasRole(PowerUserPlus) = %v, want %v", tt.role, got, tt.wantPublishing)
			}
		})
	}
}

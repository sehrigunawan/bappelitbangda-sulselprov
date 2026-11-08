package authz

import (
	"reflect"
	"testing"
)

// A1: Can matches all 45 cells of the spec matrix at View and Manage;
// Can(_, {a, None}) is false; an unknown role "root" and an unknown area "x" are denied.
func TestCanMatrix(t *testing.T) {
	t.Parallel()

	// Literal copy of docs/modules/auth.md lines 45-55 (PRD 3.4).
	// manage = Manage, view = View, — = None.
	table := []struct {
		area             Area
		superAdmin       Level
		humasEditor      Level
		documentOperator Level
		ppidOfficer      Level
		leadership       Level
	}{
		{Publications, Manage, Manage, None, None, None},
		{Pages, Manage, Manage, None, None, None},
		{Documents, Manage, None, Manage, View, None},
		{SiteContent, Manage, Manage, None, None, None},
		{Media, Manage, Manage, Manage, Manage, None},
		{Ppid, Manage, None, None, Manage, None},
		{Users, Manage, None, None, None, None},
		{ActivityLog, View, None, None, None, None},
		{Statistics, View, None, None, View, View},
	}

	for _, row := range table {
		roleLevels := []struct {
			role     Role
			maxLevel Level
		}{
			{SuperAdmin, row.superAdmin},
			{HumasEditor, row.humasEditor},
			{DocumentOperator, row.documentOperator},
			{PpidOfficer, row.ppidOfficer},
			{Leadership, row.leadership},
		}

		for _, rl := range roleLevels {
			p := Principal{UserID: 1, Roles: []Role{rl.role}}

			// Level Manage: true iff maxLevel >= Manage
			wantManage := rl.maxLevel >= Manage
			if got := Can(p, Permission{Area: row.area, Level: Manage}); got != wantManage {
				t.Errorf("Can(%s, %s: Manage) = %v, want %v", rl.role, row.area, got, wantManage)
			}

			// Level View: true iff maxLevel >= View (Manage implies View)
			wantView := rl.maxLevel >= View
			if got := Can(p, Permission{Area: row.area, Level: View}); got != wantView {
				t.Errorf("Can(%s, %s: View) = %v, want %v", rl.role, row.area, got, wantView)
			}

			// Level None: must always be false
			if got := Can(p, Permission{Area: row.area, Level: None}); got {
				t.Errorf("Can(%s, %s: None) = %v, want false", rl.role, row.area, got)
			}
		}

		// Unknown role "root" is denied for this area
		rootPrincipal := Principal{UserID: 1, Roles: []Role{"root"}}
		if Can(rootPrincipal, Permission{Area: row.area, Level: View}) {
			t.Errorf("Can(root, %s: View) = true, want false", row.area)
		}
		if Can(rootPrincipal, Permission{Area: row.area, Level: Manage}) {
			t.Errorf("Can(root, %s: Manage) = true, want false", row.area)
		}
	}

	// Unknown area "x" is denied for all valid roles
	allRoles := []Role{SuperAdmin, HumasEditor, DocumentOperator, PpidOfficer, Leadership}
	for _, r := range allRoles {
		p := Principal{UserID: 1, Roles: []Role{r}}
		if Can(p, Permission{Area: "x", Level: View}) {
			t.Errorf("Can(%s, x: View) = true, want false", r)
		}
		if Can(p, Permission{Area: "x", Level: Manage}) {
			t.Errorf("Can(%s, x: Manage) = true, want false", r)
		}
		if Can(p, Permission{Area: "x", Level: None}) {
			t.Errorf("Can(%s, x: None) = true, want false", r)
		}
	}

	// Empty and nil roles denied
	for _, emptyPrincipal := range []Principal{{UserID: 1, Roles: []Role{}}, {UserID: 1, Roles: nil}} {
		if Can(emptyPrincipal, Permission{Area: Publications, Level: View}) {
			t.Errorf("Can(empty, publications: View) = true, want false")
		}
	}
}

// A2: Union for 3 role pairs:
// 1) humas_editor + document_operator
// 2) document_operator + ppid_officer (documents = manage, so M overrides V)
// 3) ppid_officer + leadership
// Permissions are returned in area order; no roles gives a non-nil empty slice.
func TestEffectiveUnion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		roles []Role
		want  []Permission
	}{
		{
			name:  "humas_editor and document_operator",
			roles: []Role{HumasEditor, DocumentOperator},
			want: []Permission{
				{Area: Publications, Level: Manage},
				{Area: Pages, Level: Manage},
				{Area: Documents, Level: Manage},
				{Area: SiteContent, Level: Manage},
				{Area: Media, Level: Manage},
			},
		},
		{
			name:  "document_operator and ppid_officer (manage overrides view on documents)",
			roles: []Role{DocumentOperator, PpidOfficer},
			want: []Permission{
				{Area: Documents, Level: Manage},
				{Area: Media, Level: Manage},
				{Area: Ppid, Level: Manage},
				{Area: Statistics, Level: View},
			},
		},
		{
			name:  "ppid_officer and leadership",
			roles: []Role{PpidOfficer, Leadership},
			want: []Permission{
				{Area: Documents, Level: View},
				{Area: Media, Level: Manage},
				{Area: Ppid, Level: Manage},
				{Area: Statistics, Level: View},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Effective(Principal{UserID: 1, Roles: tc.roles})
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Effective(%v) =\n  got:  %v\n  want: %v", tc.roles, got, tc.want)
			}
		})
	}

	// No roles gives a non-nil empty slice
	t.Run("empty roles", func(t *testing.T) {
		got := Effective(Principal{UserID: 1, Roles: []Role{}})
		if got == nil {
			t.Fatal("Effective with empty roles returned nil, want non-nil empty slice")
		}
		if len(got) != 0 {
			t.Fatalf("Effective with empty roles returned %v, want []", got)
		}
	})

	t.Run("nil roles", func(t *testing.T) {
		got := Effective(Principal{UserID: 1, Roles: nil})
		if got == nil {
			t.Fatal("Effective with nil roles returned nil, want non-nil empty slice")
		}
		if len(got) != 0 {
			t.Fatalf("Effective with nil roles returned %v, want []", got)
		}
	})

	// Areas returns the 9 areas in table order
	t.Run("areas order", func(t *testing.T) {
		wantAreas := []Area{
			Publications,
			Pages,
			Documents,
			SiteContent,
			Media,
			Ppid,
			Users,
			ActivityLog,
			Statistics,
		}
		got := Areas()
		if !reflect.DeepEqual(got, wantAreas) {
			t.Fatalf("Areas() = %v, want %v", got, wantAreas)
		}
	})
}

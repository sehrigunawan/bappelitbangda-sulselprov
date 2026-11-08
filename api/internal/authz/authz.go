// Package authz holds the role-to-permission matrix of the auth module (docs/modules/auth.md).
package authz

// Role is a fixed role name stored in user_roles.role.
type Role string

// Roles defined by the auth module.
const (
	// SuperAdmin manages users and holds every area at manage level.
	SuperAdmin Role = "super_admin"
	// HumasEditor edits public information pages and publications.
	HumasEditor Role = "humas_editor"
	// DocumentOperator manages documents.
	DocumentOperator Role = "document_operator"
	// PpidOfficer manages PPID requests.
	PpidOfficer Role = "ppid_officer"
	// Leadership reads statistics and activity.
	Leadership Role = "leadership"
)

// Area is a permission area of the admin application.
type Area string

// Areas defined by the auth module.
const (
	// Publications covers publication content.
	Publications Area = "publications"
	// Pages covers information pages.
	Pages Area = "pages"
	// Documents covers document content.
	Documents Area = "documents"
	// SiteContent covers site-wide content.
	SiteContent Area = "site_content"
	// Media covers uploaded media.
	Media Area = "media"
	// Ppid covers PPID requests.
	Ppid Area = "ppid"
	// Users covers user administration.
	Users Area = "users"
	// ActivityLog covers the activity log.
	ActivityLog Area = "activity_log"
	// Statistics covers statistics.
	Statistics Area = "statistics"
)

// Level is the access level of a role in an area. Higher levels include lower ones.
type Level uint8

// Access levels, in increasing order.
const (
	// None grants no access.
	None Level = iota
	// View grants read access.
	View
	// Manage grants read and write access.
	Manage
)

// String returns the lowercase name of the level.
func (l Level) String() string {
	switch l {
	case View:
		return "view"
	case Manage:
		return "manage"
	default:
		return "none"
	}
}

// Permission is a level requested or granted in one area.
type Permission struct {
	Area  Area
	Level Level
}

// Principal is an authenticated user together with the roles held at the time of the check.
type Principal struct {
	UserID int64
	Roles  []Role
}

// areas lists every area in the row order of docs/modules/auth.md. It is never modified.
var areas = []Area{Publications, Pages, Documents, SiteContent, Media, Ppid, Users, ActivityLog, Statistics}

// matrix is the permission matrix of docs/modules/auth.md (PRD section 3.4). A missing entry means None.
// Changing it needs a PR; nothing outside this file writes to it.
var matrix = map[Role]map[Area]Level{
	SuperAdmin: {
		Publications: Manage, Pages: Manage, Documents: Manage, SiteContent: Manage, Media: Manage,
		Ppid: Manage, Users: Manage, ActivityLog: View, Statistics: View,
	},
	HumasEditor: {
		Publications: Manage, Pages: Manage, SiteContent: Manage, Media: Manage,
	},
	DocumentOperator: {
		Documents: Manage, Media: Manage,
	},
	PpidOfficer: {
		Documents: View, Media: Manage, Ppid: Manage, Statistics: View,
	},
	Leadership: {
		Statistics: View,
	},
}

// Can reports whether any role of the principal grants perm. An unknown role or area is denied,
// and so is a request for level None.
func Can(p Principal, perm Permission) bool {
	if perm.Level == None {
		return false
	}
	for _, r := range p.Roles {
		if matrix[r][perm.Area] >= perm.Level {
			return true
		}
	}
	return false
}

// Effective returns the highest level per area across the principal's roles, in Areas() order.
// Areas with no access are omitted. The result is never nil.
func Effective(p Principal) []Permission {
	out := []Permission{}
	for _, a := range areas {
		var best Level
		for _, r := range p.Roles {
			best = max(best, matrix[r][a])
		}
		if best > None {
			out = append(out, Permission{Area: a, Level: best})
		}
	}
	return out
}

// Areas returns every area in matrix order. The caller may modify the returned slice.
func Areas() []Area {
	return append([]Area(nil), areas...)
}

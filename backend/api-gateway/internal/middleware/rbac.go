package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Role constants define the available roles in the system.
type Role string

const (
	RoleAdmin     Role = "admin"
	RoleCreator   Role = "creator"
	RoleUser      Role = "user"
	RoleModerator Role = "moderator"
	RoleGuest     Role = "guest" // unauthenticated
)

// Permission represents a discrete action a role can perform.
type Permission string

const (
	// Video permissions
	PermVideoUpload   Permission = "video:upload"
	PermVideoDelete   Permission = "video:delete"
	PermVideoModerate Permission = "video:moderate"
	PermVideoView     Permission = "video:view"

	// User permissions
	PermUserRead      Permission = "user:read"
	PermUserWrite     Permission = "user:write"
	PermUserDelete    Permission = "user:delete"
	PermUserBan       Permission = "user:ban"

	// Comment permissions
	PermCommentCreate Permission = "comment:create"
	PermCommentDelete Permission = "comment:delete"
	PermCommentModerate Permission = "comment:moderate"

	// Analytics permissions
	PermAnalyticsRead  Permission = "analytics:read"
	PermAnalyticsWrite Permission = "analytics:write"

	// Admin permissions
	PermSystemConfig  Permission = "system:config"
	PermAuditRead     Permission = "audit:read"
	PermRoleAssign    Permission = "role:assign"

	// Live permissions
	PermLiveStart     Permission = "live:start"
	PermLiveModerate  Permission = "live:moderate"
)

// rolePermissions maps each role to its granted permissions. Higher-privilege
// roles are granted all permissions of lower roles via expandRolePermissions.
var rolePermissions = map[Role][]Permission{
	RoleGuest: {
		PermVideoView,
		PermUserRead,
	},
	RoleUser: {
		PermVideoView,
		PermUserRead,
		PermUserWrite,
		PermCommentCreate,
		PermCommentDelete, // own comments only — enforced at service layer
	},
	RoleCreator: {
		PermVideoView,
		PermVideoUpload,
		PermVideoDelete, // own videos only — enforced at service layer
		PermUserRead,
		PermUserWrite,
		PermCommentCreate,
		PermCommentDelete,
		PermAnalyticsRead,
		PermLiveStart,
	},
	RoleModerator: {
		PermVideoView,
		PermVideoUpload,
		PermVideoDelete,
		PermVideoModerate,
		PermUserRead,
		PermUserWrite,
		PermUserBan,
		PermCommentCreate,
		PermCommentDelete,
		PermCommentModerate,
		PermAnalyticsRead,
		PermLiveStart,
		PermLiveModerate,
	},
	RoleAdmin: {
		PermVideoView,
		PermVideoUpload,
		PermVideoDelete,
		PermVideoModerate,
		PermUserRead,
		PermUserWrite,
		PermUserDelete,
		PermUserBan,
		PermCommentCreate,
		PermCommentDelete,
		PermCommentModerate,
		PermAnalyticsRead,
		PermAnalyticsWrite,
		PermSystemConfig,
		PermAuditRead,
		PermRoleAssign,
		PermLiveStart,
		PermLiveModerate,
	},
}

// permissionSet is a precomputed set for O(1) lookup.
type permissionSet map[Permission]struct{}

// expandedPermissions holds the pre-computed permission sets per role.
var expandedPermissions map[Role]permissionSet

func init() {
	expandedPermissions = make(map[Role]permissionSet, len(rolePermissions))
	for role, perms := range rolePermissions {
		set := make(permissionSet, len(perms))
		for _, p := range perms {
			set[p] = struct{}{}
		}
		expandedPermissions[role] = set
	}
}

// RBACMiddleware provides role-based access control middleware factories.
type RBACMiddleware struct{}

// NewRBACMiddleware creates a new RBAC middleware instance.
func NewRBACMiddleware() *RBACMiddleware {
	return &RBACMiddleware{}
}

// RequireRoles returns a middleware that allows access only if the authenticated
// user has one of the specified roles. Must be used after ValidateJWT.
func (r *RBACMiddleware) RequireRoles(roles ...Role) gin.HandlerFunc {
	// Pre-build a set of allowed roles for O(1) lookup.
	allowed := make(map[Role]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}

	return func(c *gin.Context) {
		userRole := extractRole(c)
		if _, ok := allowed[userRole]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "you do not have the required role to access this resource",
				"required_roles": rolesToStrings(roles),
				"your_role":      string(userRole),
			})
			return
		}
		c.Next()
	}
}

// RequirePermission returns a middleware that checks whether the authenticated
// user's role grants the requested permission.
func (r *RBACMiddleware) RequirePermission(perm Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := extractRole(c)
		if !hasPermission(userRole, perm) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":      "forbidden",
				"message":    "insufficient permissions",
				"permission": string(perm),
			})
			return
		}
		c.Next()
	}
}

// RequireAnyPermission returns a middleware that passes if the user has at
// least one of the given permissions.
func (r *RBACMiddleware) RequireAnyPermission(perms ...Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := extractRole(c)
		for _, perm := range perms {
			if hasPermission(userRole, perm) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "insufficient permissions",
		})
	}
}

// RequireAllPermissions returns a middleware that passes only if the user holds
// every one of the given permissions.
func (r *RBACMiddleware) RequireAllPermissions(perms ...Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := extractRole(c)
		for _, perm := range perms {
			if !hasPermission(userRole, perm) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error":   "forbidden",
					"message": "insufficient permissions",
					"missing": string(perm),
				})
				return
			}
		}
		c.Next()
	}
}

// AdminOnly is a convenience wrapper for RequireRoles(RoleAdmin).
func (r *RBACMiddleware) AdminOnly() gin.HandlerFunc {
	return r.RequireRoles(RoleAdmin)
}

// ModeratorOrAbove allows moderators and admins.
func (r *RBACMiddleware) ModeratorOrAbove() gin.HandlerFunc {
	return r.RequireRoles(RoleModerator, RoleAdmin)
}

// CreatorOrAbove allows creators, moderators, and admins.
func (r *RBACMiddleware) CreatorOrAbove() gin.HandlerFunc {
	return r.RequireRoles(RoleCreator, RoleModerator, RoleAdmin)
}

// AuthenticatedOnly rejects unauthenticated (guest) requests.
func (r *RBACMiddleware) AuthenticatedOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := extractRole(c)
		if userRole == RoleGuest {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "authentication required",
			})
			return
		}
		c.Next()
	}
}

// SelfOrAdmin allows the request only if the authenticated user is accessing
// their own resource (userID matches the :id path param) or is an admin.
func (r *RBACMiddleware) SelfOrAdmin(paramName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := extractRole(c)
		if userRole == RoleAdmin {
			c.Next()
			return
		}

		userID, exists := c.Get(GinKeyUserID)
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "authentication required",
			})
			return
		}

		resourceID := c.Param(paramName)
		if resourceID == "" {
			resourceID = c.Query(paramName)
		}

		if userID.(string) != resourceID {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "you can only access your own resources",
			})
			return
		}

		c.Next()
	}
}

// --- helpers ---

func extractRole(c *gin.Context) Role {
	rawRole, exists := c.Get(GinKeyRole)
	if !exists {
		return RoleGuest
	}
	roleStr, ok := rawRole.(string)
	if !ok || roleStr == "" {
		return RoleGuest
	}
	role := Role(strings.ToLower(roleStr))
	if _, valid := rolePermissions[role]; !valid {
		return RoleUser // default authenticated role for unknown role strings
	}
	return role
}

func hasPermission(role Role, perm Permission) bool {
	set, ok := expandedPermissions[role]
	if !ok {
		return false
	}
	_, ok = set[perm]
	return ok
}

func rolesToStrings(roles []Role) []string {
	result := make([]string, len(roles))
	for i, r := range roles {
		result[i] = string(r)
	}
	return result
}

// GetRolePermissions returns all permissions for a given role (used for introspection endpoints).
func GetRolePermissions(role Role) []Permission {
	return rolePermissions[role]
}

// HasPermission is the exported version for use outside middleware.
func HasPermission(role Role, perm Permission) bool {
	return hasPermission(role, perm)
}

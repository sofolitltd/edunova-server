package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"edunova-server/database"
)

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			c.Abort()
			return
		}

		token := parts[1]

		email, valid := ValidateAdminToken(token)
		if !valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired admin token"})
			c.Abort()
			return
		}

		var role string
		var adminID int
		_ = database.DB.QueryRow(context.Background(),
			`SELECT id, COALESCE(role, 'admin') FROM admin_users WHERE email = $1`, email,
		).Scan(&adminID, &role)

		c.Set("admin_id", adminID)
		c.Set("admin_email", email)
		c.Set("admin_role", role)
		c.Next()
	}
}

func MasterRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("admin_role")
		if !exists || role.(string) != "master_admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "master admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func TeacherRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			c.Abort()
			return
		}

		email, valid := ValidateTeacherToken(parts[1])
		if !valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired teacher token"})
			c.Abort()
			return
		}

		var teacherID int
		_ = database.DB.QueryRow(context.Background(),
			`SELECT id FROM teachers WHERE email = $1`, email,
		).Scan(&teacherID)
		if teacherID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "teacher account not found"})
			c.Abort()
			return
		}

		c.Set("teacher_id", teacherID)
		c.Set("teacher_email", email)
		c.Set("actor_type", "teacher")
		c.Next()
	}
}

// StaffRequired accepts either a valid admin token or a valid teacher token,
// so routes shared between the admin dashboard and the independent teacher
// portal (attendance, lessons, exams, etc.) can be reached by both, while
// staying fully separate identities (different tables, different JWTs).
func StaffRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			c.Abort()
			return
		}
		token := parts[1]

		if email, valid := ValidateAdminToken(token); valid {
			var role string
			var adminID int
			_ = database.DB.QueryRow(context.Background(),
				`SELECT id, COALESCE(role, 'admin') FROM admin_users WHERE email = $1`, email,
			).Scan(&adminID, &role)
			if adminID == 0 {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "admin account not found"})
				c.Abort()
				return
			}
			c.Set("admin_id", adminID)
			c.Set("admin_email", email)
			c.Set("admin_role", role)
			c.Set("actor_type", "admin")
			c.Next()
			return
		}

		if email, valid := ValidateTeacherToken(token); valid {
			var teacherID int
			_ = database.DB.QueryRow(context.Background(),
				`SELECT id FROM teachers WHERE email = $1`, email,
			).Scan(&teacherID)
			if teacherID == 0 {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "teacher account not found"})
				c.Abort()
				return
			}
			c.Set("teacher_id", teacherID)
			c.Set("teacher_email", email)
			c.Set("actor_type", "teacher")
			c.Next()
			return
		}

		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		c.Abort()
	}
}

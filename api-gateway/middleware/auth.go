package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/qiwang/book-e-commerce-micro/common/auth"
	"github.com/qiwang/book-e-commerce-micro/common/util"
)

func AuthMiddleware(jwtMgr *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			util.Unauthorized(c, "missing authorization header")
			c.Abort()
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		if token == header {
			util.Unauthorized(c, "invalid authorization format")
			c.Abort()
			return
		}

		claims, err := jwtMgr.ValidateToken(token)
		if err != nil {
			util.Unauthorized(c, "invalid or expired token")
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		c.Next()
	}
}

func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != "admin" {
			util.Forbidden(c, "admin access required")
			c.Abort()
			return
		}
		c.Next()
	}
}

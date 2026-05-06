package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const UserIDKey = "userID"
const OrgIDKey = "orgID"
const RoleKey = "role"

func Auth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				return c.JSON(http.StatusUnauthorized, echo.Map{"error": "missing token"})
			}
			tokenStr := strings.TrimPrefix(header, "Bearer ")

			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
				return []byte(jwtSecret()), nil
			}, jwt.WithValidMethods([]string{"HS256"}))
			if err != nil || !token.Valid {
				return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid token"})
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid claims"})
			}

			sub, err := claims.GetSubject()
			if err != nil {
				return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid subject"})
			}
			userID, err := uuid.Parse(sub)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid user id"})
			}

			orgIDStr, _ := claims["org_id"].(string)
			orgID, err := uuid.Parse(orgIDStr)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid org id"})
			}

			role, _ := claims["role"].(string)

			c.Set(UserIDKey, userID)
			c.Set(OrgIDKey, orgID)
			c.Set(RoleKey, role)
			return next(c)
		}
	}
}

func jwtSecret() string {
	s := os.Getenv("JWT_SECRET")
	if s == "" {
		return "changeme-set-JWT_SECRET-in-env"
	}
	return s
}

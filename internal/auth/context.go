package auth

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const ContextAuthPrincipalKey = "auth.principal"

// GetUserID retrieves the authenticated UserID from the Gin context.
func GetUserID(c *gin.Context) (uuid.UUID, error) {
	raw, exists := c.Get(ContextAuthPrincipalKey)
	if !exists {
		return uuid.Nil, fmt.Errorf("auth principal missing from context")
	}

	principal, ok := raw.(AuthPrincipal)
	if !ok {
		return uuid.Nil, fmt.Errorf("invalid auth principal type")
	}

	userID, err := uuid.Parse(principal.UserID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid user id in principal: %w", err)
	}

	return userID, nil
}

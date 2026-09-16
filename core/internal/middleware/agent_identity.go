package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/fortuna/core/pkg/agentidentity"
	"github.com/gin-gonic/gin"
)

const agentPrincipalKey = "fortuna.agent.principal"

// AgentIngestAuth selects one explicit migration mode. A non-empty registry path
// always enables scoped agent identity and never falls back to the shared token.
func AgentIngestAuth(registryPath, legacyToken string) gin.HandlerFunc {
	if strings.TrimSpace(registryPath) != "" {
		return RequireAgentIdentity(agentidentity.Store{Path: strings.TrimSpace(registryPath)})
	}
	return RequireIngestToken(legacyToken)
}

// RequireAgentIdentity authenticates HTTP agent ingest with a credential that is
// explicitly bound to one agent and one cluster.
func RequireAgentIdentity(store agentidentity.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := agentCredentialFromRequest(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "agent authentication required", "code": "agent_auth_required"})
			return
		}
		principal, err := store.AuthenticateToken(token)
		if err != nil {
			if errors.Is(err, agentidentity.ErrUnavailable) {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "agent authentication unavailable", "code": "agent_auth_unavailable"})
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "agent authentication required", "code": "agent_auth_rejected"})
			return
		}
		c.Set(agentPrincipalKey, principal)
		c.Next()
	}
}

// AgentPrincipal returns only a principal created by RequireAgentIdentity.
func AgentPrincipal(c *gin.Context) (agentidentity.Principal, bool) {
	value, ok := c.Get(agentPrincipalKey)
	if !ok {
		return agentidentity.Principal{}, false
	}
	principal, ok := value.(agentidentity.Principal)
	if !ok || principal.CredentialID == "" || principal.ClusterID == "" || principal.AgentID == "" {
		return agentidentity.Principal{}, false
	}
	return principal, true
}

func agentCredentialFromRequest(c *gin.Context) string {
	if token := strings.TrimSpace(c.GetHeader(ingestTokenHeader)); token != "" {
		return token
	}
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	if len(auth) > 7 && strings.EqualFold(auth[:7], "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return ""
}

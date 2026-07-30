package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/security"
	"github.com/fortuna/core/pkg/securityaudit"
)

// CertHandler handles certificate management API
type CertHandler struct {
	certManager *security.CertManager
}

// NewCertHandler creates a new certificate handler
func NewCertHandler(certManager *security.CertManager) *CertHandler {
	return &CertHandler{
		certManager: certManager,
	}
}

// GetCertificateInfo returns certificate information
func (h *CertHandler) GetCertificateInfo(c *gin.Context) {
	info, err := h.certManager.GetCertificateInfo()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get certificate",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"subject":          info.Subject,
		"issuer":           info.Issuer,
		"serial_number":    info.SerialNumber,
		"not_before":       info.NotBefore.Format(time.RFC3339),
		"not_after":        info.NotAfter.Format(time.RFC3339),
		"days_until_expiry": info.DaysUntilExpiry,
		"is_expired":       info.IsExpired,
		"dns_names":        info.DNSNames,
	})
}

// RotateCertificateHandler triggers certificate rotation and records governance audit (requires DB for audit pipeline).
func RotateCertificateHandler(db *gorm.DB, cm *security.CertManager) gin.HandlerFunc {
	h := NewCertHandler(cm)
	return func(c *gin.Context) {
		if err := h.certManager.RotateCertificate(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Certificate rotation failed",
				"details": err.Error(),
			})
			return
		}
		ev := securityaudit.FromRequest(
			c,
			authorization.ToStrings(middleware.GrantedPermissions(c)),
			c.GetString(middleware.CtxJWTSessionID),
			"cluster_certificate_rotate",
			"cluster",
			"tls",
			"success",
			"high",
			"jwt",
			nil,
			map[string]any{"rotated": true},
			nil,
			nil,
		)
		securityaudit.Append(db, &ev)
		c.JSON(http.StatusOK, gin.H{
			"message": "Certificate rotated successfully",
		})
	}
}

// RotateCertificate triggers certificate rotation
func (h *CertHandler) RotateCertificate(c *gin.Context) {
	if err := h.certManager.RotateCertificate(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Certificate rotation failed",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Certificate rotated successfully",
	})
}

// GetCertificateRotationHistory returns certificate rotation history.
// Returns empty list until rotation_history table exists; no mock data.
func GetCertificateRotationHistory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"history": []map[string]interface{}{},
			"total":   0,
		})
	}
}




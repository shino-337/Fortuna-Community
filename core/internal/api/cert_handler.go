package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ksam/core/pkg/security"
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



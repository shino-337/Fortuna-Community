package security

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/ksam/core/pkg/metrics"
)

// CertManager manages certificate loading and rotation
type CertManager struct {
	certPath   string
	keyPath    string
	caCertPath string

	cert     *tls.Certificate
	certMu   sync.RWMutex
	caCert   *x509.CertPool
	caCertMu sync.RWMutex

	stopChan           chan struct{}
	expiryCheckInterval time.Duration
}

// NewCertManager creates a new certificate manager
func NewCertManager(certPath, keyPath, caCertPath string) (*CertManager, error) {
	cm := &CertManager{
		certPath:            certPath,
		keyPath:             keyPath,
		caCertPath:          caCertPath,
		stopChan:            make(chan struct{}),
		expiryCheckInterval: 1 * time.Hour, // Check expiry every hour
	}

	// Load initial certificate
	if err := cm.LoadCertificate(); err != nil {
		return nil, fmt.Errorf("failed to load initial certificate: %w", err)
	}

	// Load CA certificate
	if err := cm.LoadCACertificate(); err != nil {
		return nil, fmt.Errorf("failed to load CA certificate: %w", err)
	}

	return cm, nil
}

// LoadCertificate loads the certificate and key from disk
func (cm *CertManager) LoadCertificate() error {
	log.Printf("[CertManager] Loading certificate from %s, key from %s", cm.certPath, cm.keyPath)

	cert, err := tls.LoadX509KeyPair(cm.certPath, cm.keyPath)
	if err != nil {
		return fmt.Errorf("failed to load certificate/key: %w", err)
	}

	// Parse certificate to get expiry info
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	daysUntilExpiry := time.Until(x509Cert.NotAfter).Hours() / 24
	log.Printf("[CertManager] Certificate loaded: Subject=%s, Expires in %.0f days", 
		x509Cert.Subject.String(), daysUntilExpiry)

	// Update certificate atomically
	cm.certMu.Lock()
	cm.cert = &cert
	cm.certMu.Unlock()

	// Update metrics
	metrics.CertExpiryTime.Set(float64(x509Cert.NotAfter.Unix()))
	metrics.CertDaysUntilExpiry.Set(daysUntilExpiry)

	// Log warnings if expiring soon
	if daysUntilExpiry < 30 {
		log.Printf("[CertManager] ⚠️  WARNING: Certificate expires in %.0f days", daysUntilExpiry)
		metrics.CertExpiryWarningTotal.Inc()
	}
	if daysUntilExpiry < 7 {
		log.Printf("[CertManager] 🚨 CRITICAL: Certificate expires in %.0f days!", daysUntilExpiry)
		metrics.CertExpiryCriticalTotal.Inc()
	}
	if time.Now().After(x509Cert.NotAfter) {
		log.Printf("[CertManager] ❌ EXPIRED: Certificate has expired!")
		metrics.CertExpiredTotal.Inc()
	}

	return nil
}

// LoadCACertificate loads the CA certificate from disk
func (cm *CertManager) LoadCACertificate() error {
	log.Printf("[CertManager] Loading CA certificate from %s", cm.caCertPath)

	caCertData, err := os.ReadFile(cm.caCertPath)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCertData) {
		return fmt.Errorf("failed to parse CA certificate")
	}

	cm.caCertMu.Lock()
	cm.caCert = caCertPool
	cm.caCertMu.Unlock()

	log.Printf("[CertManager] CA certificate loaded successfully")
	return nil
}

// GetTLSCertificate returns the current certificate for TLS config
// This is used as a callback for dynamic certificate loading
func (cm *CertManager) GetTLSCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cm.certMu.RLock()
	defer cm.certMu.RUnlock()

	if cm.cert == nil {
		return nil, fmt.Errorf("certificate not loaded")
	}

	return cm.cert, nil
}

// GetCertificate returns the current certificate
func (cm *CertManager) GetCertificate() (*tls.Certificate, error) {
	cm.certMu.RLock()
	defer cm.certMu.RUnlock()

	if cm.cert == nil {
		return nil, fmt.Errorf("certificate not loaded")
	}

	return cm.cert, nil
}

// GetCACertPool returns the current CA certificate pool
func (cm *CertManager) GetCACertPool() *x509.CertPool {
	cm.caCertMu.RLock()
	defer cm.caCertMu.RUnlock()

	return cm.caCert
}

// GetCertificateInfo returns detailed certificate information
func (cm *CertManager) GetCertificateInfo() (*CertificateInfo, error) {
	cm.certMu.RLock()
	defer cm.certMu.RUnlock()

	if cm.cert == nil {
		return nil, fmt.Errorf("certificate not loaded")
	}

	x509Cert, err := x509.ParseCertificate(cm.cert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	daysUntilExpiry := time.Until(x509Cert.NotAfter).Hours() / 24

	return &CertificateInfo{
		Subject:        x509Cert.Subject.String(),
		Issuer:         x509Cert.Issuer.String(),
		SerialNumber:   x509Cert.SerialNumber.String(),
		NotBefore:      x509Cert.NotBefore,
		NotAfter:       x509Cert.NotAfter,
		DaysUntilExpiry: int(daysUntilExpiry),
		IsExpired:      time.Now().After(x509Cert.NotAfter),
		DNSNames:       x509Cert.DNSNames,
	}, nil
}

// CertificateInfo contains certificate metadata
type CertificateInfo struct {
	Subject         string    `json:"subject"`
	Issuer          string    `json:"issuer"`
	SerialNumber    string    `json:"serial_number"`
	NotBefore       time.Time `json:"not_before"`
	NotAfter        time.Time `json:"not_after"`
	DaysUntilExpiry int       `json:"days_until_expiry"`
	IsExpired       bool      `json:"is_expired"`
	DNSNames        []string  `json:"dns_names"`
}

// StartExpiryMonitoring starts a background goroutine to monitor certificate expiry
func (cm *CertManager) StartExpiryMonitoring() {
	log.Printf("[CertManager] Starting certificate expiry monitoring (check interval: %v)", cm.expiryCheckInterval)

	go func() {
		ticker := time.NewTicker(cm.expiryCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				cm.checkExpiry()
			case <-cm.stopChan:
				log.Printf("[CertManager] Stopping certificate expiry monitoring")
				return
			}
		}
	}()
}

// checkExpiry checks certificate expiry and logs warnings
func (cm *CertManager) checkExpiry() {
	info, err := cm.GetCertificateInfo()
	if err != nil {
		log.Printf("[CertManager] ERROR: Failed to check certificate expiry: %v", err)
		return
	}

	daysUntilExpiry := float64(info.DaysUntilExpiry)

	// Update metrics
	metrics.CertDaysUntilExpiry.Set(daysUntilExpiry)

	// Log warnings based on expiry time
	if daysUntilExpiry < 30 && daysUntilExpiry > 7 {
		log.Printf("[CertManager] ⚠️  WARNING: Certificate expires in %.0f days", daysUntilExpiry)
		metrics.CertExpiryWarningTotal.Inc()
	} else if daysUntilExpiry < 7 && daysUntilExpiry > 0 {
		log.Printf("[CertManager] 🚨 CRITICAL: Certificate expires in %.0f days!", daysUntilExpiry)
		metrics.CertExpiryCriticalTotal.Inc()
	} else if info.IsExpired {
		log.Printf("[CertManager] ❌ EXPIRED: Certificate has expired!")
		metrics.CertExpiredTotal.Inc()
	}
}

// RotateCertificate reloads the certificate from disk
// This should be called when cert-manager or external tool renews the certificate
func (cm *CertManager) RotateCertificate() error {
	log.Printf("[CertManager] ======================================")
	log.Printf("[CertManager] Rotating certificate...")

	start := time.Now()

	// Reload certificate from disk
	if err := cm.LoadCertificate(); err != nil {
		log.Printf("[CertManager] ❌ ERROR: Certificate rotation failed: %v", err)
		metrics.CertRotationFailureTotal.Inc()
		return err
	}

	duration := time.Since(start)

	log.Printf("[CertManager] ✅ Certificate rotated successfully in %v", duration)
	log.Printf("[CertManager] ======================================")

	// Update metrics
	metrics.CertRotationTotal.Inc()
	metrics.CertRotationDuration.Observe(duration.Seconds())
	metrics.LastCertRotationTime.Set(float64(time.Now().Unix()))

	return nil
}

// Stop stops the certificate manager
func (cm *CertManager) Stop() {
	close(cm.stopChan)
}


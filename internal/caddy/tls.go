package caddy

import (
	"fmt"

	"github.com/posternproxy/posternproxy/internal/models"
)

// BuildTLSApp generates the apps.tls section from all certificates.
// When agentCertDir is non-empty (remote server builds), predictable agent-side
// paths are used instead of the controller's local filesystem paths.
func BuildTLSApp(certs []*models.Certificate, agentCertDir string) (*TLSApp, error) {
	app := &TLSApp{
		Certificates: &TLSCertificates{},
	}

	for _, cert := range certs {
		if cert.Provider != "custom" {
			continue
		}
		if cert.CertPath == nil || cert.KeyPath == nil {
			continue
		}

		var certPath, keyPath string
		if agentCertDir != "" {
			certPath = fmt.Sprintf("%s/%d/cert.pem", agentCertDir, cert.ID)
			keyPath = fmt.Sprintf("%s/%d/key.pem", agentCertDir, cert.ID)
		} else {
			certPath = *cert.CertPath
			keyPath = *cert.KeyPath
		}

		app.Certificates.LoadFiles = append(app.Certificates.LoadFiles, CertFileLoader{
			Certificate: certPath,
			Key:         keyPath,
			Tags:        []string{fmt.Sprintf("cert_%d", cert.ID)},
		})
	}

	return app, nil
}

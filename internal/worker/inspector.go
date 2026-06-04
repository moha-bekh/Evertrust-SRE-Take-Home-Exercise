package worker

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"

	"certificate-inspector/internal/job"
)

type CertificateInspector struct {
	timeout time.Duration
}

func NewCertificateInspector(timeout time.Duration) *CertificateInspector {
	return &CertificateInspector{timeout: timeout}
}

func (i *CertificateInspector) Inspect(ctx context.Context, hostname string, port int) (job.CertificateResult, error) {
	dialer := &net.Dialer{Timeout: i.timeout}
	address := net.JoinHostPort(hostname, strconv.Itoa(port))

	conn, err := tls.DialWithDialer(dialer, "tcp", address, &tls.Config{
		ServerName: hostname,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return job.CertificateResult{}, fmt.Errorf("tls connection failed: %w", err)
	}
	defer conn.Close()

	select {
	case <-ctx.Done():
		return job.CertificateResult{}, ctx.Err()
	default:
	}

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return job.CertificateResult{}, fmt.Errorf("no peer certificates returned")
	}

	leaf := state.PeerCertificates[0]
	now := time.Now().UTC()
	return job.CertificateResult{
		Hostname:           hostname,
		Port:               port,
		SubjectCommonName:  leaf.Subject.CommonName,
		IssuerCommonName:   leaf.Issuer.CommonName,
		DNSNames:           leaf.DNSNames,
		NotBefore:          leaf.NotBefore.UTC(),
		NotAfter:           leaf.NotAfter.UTC(),
		DaysRemaining:      int(leaf.NotAfter.Sub(now).Hours() / 24),
		IsExpired:          now.After(leaf.NotAfter),
		SignatureAlgorithm: leaf.SignatureAlgorithm.String(),
		PublicKeyAlgorithm: leaf.PublicKeyAlgorithm.String(),
		SerialNumber:       leaf.SerialNumber.String(),
		TLSVersion:         tlsVersion(state.Version),
	}, nil
}

func tlsVersion(version uint16) string {
	switch version {
	case tls.VersionTLS13:
		return "TLS 1.3"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS10:
		return "TLS 1.0"
	default:
		return "unknown"
	}
}

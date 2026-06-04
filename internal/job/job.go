package job

import "time"

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

type Job struct {
	ID             string
	Hostname       string
	Port           int
	Status         Status
	Attempts       int
	MaxAttempts    int
	Error          string
	ResultJSON     string
	IdempotencyKey string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
}

type CertificateResult struct {
	Hostname           string    `json:"hostname"`
	Port               int       `json:"port"`
	SubjectCommonName  string    `json:"subject_common_name"`
	IssuerCommonName   string    `json:"issuer_common_name"`
	DNSNames           []string  `json:"dns_names"`
	NotBefore          time.Time `json:"not_before"`
	NotAfter           time.Time `json:"not_after"`
	DaysRemaining      int       `json:"days_remaining"`
	IsExpired          bool      `json:"is_expired"`
	SignatureAlgorithm string    `json:"signature_algorithm"`
	PublicKeyAlgorithm string    `json:"public_key_algorithm"`
	SerialNumber       string    `json:"serial_number"`
	TLSVersion         string    `json:"tls_version"`
}

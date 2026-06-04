package api

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"certificate-inspector/internal/job"
)

var hostnamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)

type createJobRequest struct {
	Hostname       string `json:"hostname"`
	Port           int    `json:"port"`
	IdempotencyKey string `json:"idempotency_key"`
}

type createJobResponse struct {
	ID       string     `json:"id"`
	Status   job.Status `json:"status"`
	Hostname string     `json:"hostname"`
	Port     int        `json:"port"`
}

type statusResponse struct {
	ID        string     `json:"id"`
	Status    job.Status `json:"status"`
	Attempts  int        `json:"attempts"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func validateCreateJob(req createJobRequest) error {
	if req.Hostname == "" {
		return errors.New("invalid hostname")
	}
	if len(req.Hostname) > 253 {
		return errors.New("invalid hostname")
	}
	if strings.Contains(req.Hostname, "://") {
		return errors.New("hostname must not include a scheme")
	}
	if net.ParseIP(req.Hostname) != nil {
		return errors.New("hostname must be a dns name")
	}
	if !hostnamePattern.MatchString(req.Hostname) {
		return errors.New("invalid hostname")
	}
	if req.Port < 0 || req.Port > 65535 {
		return errors.New("invalid port")
	}
	if len(req.IdempotencyKey) > 128 {
		return errors.New("invalid idempotency key")
	}
	return nil
}

func responseFromJob(item job.Job) createJobResponse {
	return createJobResponse{
		ID:       item.ID,
		Status:   item.Status,
		Hostname: item.Hostname,
		Port:     item.Port,
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

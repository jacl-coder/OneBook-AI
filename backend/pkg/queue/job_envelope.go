package queue

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// JobEnvelope is the payload written to Kafka.
type JobEnvelope struct {
	JobID        string          `json:"jobId"`
	JobType      string          `json:"jobType"`
	ResourceType string          `json:"resourceType"`
	ResourceID   string          `json:"resourceId"`
	Attempt      int             `json:"attempt"`
	RequestedAt  time.Time       `json:"requestedAt"`
	TraceID      string          `json:"traceId,omitempty"`
	Payload      json.RawMessage `json:"payload,omitempty"`
}

func (e JobEnvelope) Encode() ([]byte, error) {
	return json.Marshal(e)
}

func DecodeJobEnvelope(data []byte) (JobEnvelope, error) {
	var env JobEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return JobEnvelope{}, fmt.Errorf("decode job envelope: %w", err)
	}
	if strings.TrimSpace(env.JobID) == "" {
		return JobEnvelope{}, fmt.Errorf("decode job envelope: jobId required")
	}
	if strings.TrimSpace(env.JobType) == "" {
		return JobEnvelope{}, fmt.Errorf("decode job envelope: jobType required")
	}
	if strings.TrimSpace(env.ResourceType) == "" {
		return JobEnvelope{}, fmt.Errorf("decode job envelope: resourceType required")
	}
	if strings.TrimSpace(env.ResourceID) == "" {
		return JobEnvelope{}, fmt.Errorf("decode job envelope: resourceId required")
	}
	return env, nil
}

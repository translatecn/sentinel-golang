package isolation

import (
	"encoding/json"
	"fmt"
)

// MetricType represents the target metric type.
type MetricType int32

const (
	// Concurrency represents concurrency (in-flight requests).
	Concurrency MetricType = iota
)

func (s MetricType) String() string {
	switch s {
	case Concurrency:
		return "Concurrency"
	default:
		return "Undefined"
	}
}

// Rule 描述隔离策略(例如，信号量隔离)。
type Rule struct {
	// ID represents the unique ID of the rule (optional).
	ID string `json:"id,omitempty"`
	// Resource represents the target resource definition.
	Resource string `json:"resource"`
	// MetricType indicates the metric type for checking logic.
	// Currently Concurrency is supported for concurrency limiting.
	MetricType MetricType `json:"metricType"`
	Threshold  uint32     `json:"threshold"`
}

func (r *Rule) String() string {
	b, err := json.Marshal(r)
	if err != nil {
		// Return the fallback string
		return fmt.Sprintf("{Id=%s, Resource=%s, MetricType=%s, Threshold=%d}", r.ID, r.Resource, r.MetricType.String(), r.Threshold)
	}
	return string(b)
}

func (r *Rule) ResourceName() string {
	return r.Resource
}

package isolation

import (
	"encoding/json"
	"fmt"
)

type MetricType int32

const (
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

// Rule 描述隔离策略(例如，信号量隔离).
type Rule struct {
	ID         string     `json:"id,omitempty"`
	Resource   string     `json:"resource"`
	MetricType MetricType `json:"metricType"`
	Threshold  uint32     `json:"threshold"`
}

func (r *Rule) String() string {
	b, err := json.Marshal(r)
	if err != nil {
		return fmt.Sprintf("{Id=%s, Resource=%s, MetricType=%s, Threshold=%d}", r.ID, r.Resource, r.MetricType.String(), r.Threshold)
	}
	return string(b)
}

func (r *Rule) ResourceName() string {
	return r.Resource
}

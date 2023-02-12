package hotspot

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
)

type ControlBehavior int32

const (
	Reject ControlBehavior = iota
	Throttling
)

func (t ControlBehavior) String() string {
	switch t {
	case Reject:
		return "Reject"
	case Throttling:
		return "Throttling"
	default:
		return strconv.Itoa(int(t))
	}
}

type MetricType int32

const (
	Concurrency MetricType = iota
	QPS
)

func (t MetricType) String() string {
	switch t {
	case Concurrency:
		return "Concurrency"
	case QPS:
		return "QPS"
	default:
		return "Undefined"
	}
}

type Rule struct {
	ID              string          `json:"id,omitempty"`
	Resource        string          `json:"resource"`
	MetricType      MetricType      `json:"metricType"`
	ControlBehavior ControlBehavior `json:"controlBehavior"`
	// ParamIndex is the index in context arguments slice.
	// if ParamIndex is great than or equals to zero, ParamIndex means the <ParamIndex>-th parameter
	// if ParamIndex is the negative, ParamIndex means the reversed <ParamIndex>-th parameter
	ParamIndex int `json:"paramIndex"`
	// ParamKey is the key in EntryContext.Input.Attachments map.
	// ParamKey can be used as a supplement to ParamIndex to facilitate rules to quickly obtain parameter from a large number of parameters
	// ParamKey is mutually exclusive with ParamIndex, ParamKey has the higher priority than ParamIndex
	ParamKey string `json:"paramKey"`
	// Threshold is the threshold to trigger rejection
	Threshold int64 `json:"threshold"`
	// MaxQueueingTimeMs only takes effect when ControlBehavior is Throttling and MetricType is QPS
	MaxQueueingTimeMs int64 `json:"maxQueueingTimeMs"`
	// BurstCount is the silent count
	// BurstCount only takes effect when ControlBehavior is Reject and MetricType is QPS
	BurstCount int64 `json:"burstCount"`
	// DurationInSec is the time interval in statistic
	// DurationInSec only takes effect when MetricType is QPS
	DurationInSec     int64                 `json:"durationInSec"`
	ParamsMaxCapacity int64                 `json:"paramsMaxCapacity"` // cache 最大容量
	SpecificItems     map[interface{}]int64 `json:"specificItems"`     // 特定值的特殊阈值
}

func (r *Rule) String() string {
	b, err := json.Marshal(r)
	if err != nil {
		// Return the fallback string
		return fmt.Sprintf("{Id:%s, Resource:%s, MetricType:%+v, ControlBehavior:%+v, ParamIndex:%d, ParamKey:%s, Threshold:%d, MaxQueueingTimeMs:%d, BurstCount:%d, DurationInSec:%d, ParamsMaxCapacity:%d, SpecificItems:%+v}",
			r.ID, r.Resource, r.MetricType, r.ControlBehavior, r.ParamIndex, r.ParamKey, r.Threshold, r.MaxQueueingTimeMs, r.BurstCount, r.DurationInSec, r.ParamsMaxCapacity, r.SpecificItems)
	}
	return string(b)
}
func (r *Rule) ResourceName() string {
	return r.Resource
}

// IsStatReusable checks whether current rule is "statistically" equal to the given rule.
func (r *Rule) IsStatReusable(newRule *Rule) bool {
	return r.Resource == newRule.Resource && r.ControlBehavior == newRule.ControlBehavior && r.ParamsMaxCapacity == newRule.ParamsMaxCapacity && r.DurationInSec == newRule.DurationInSec && r.MetricType == newRule.MetricType
}

func (r *Rule) Equals(newRule *Rule) bool {
	baseCheck := r.Resource == newRule.Resource && r.MetricType == newRule.MetricType && r.ControlBehavior == newRule.ControlBehavior && r.ParamsMaxCapacity == newRule.ParamsMaxCapacity && r.ParamIndex == newRule.ParamIndex && r.ParamKey == newRule.ParamKey && r.Threshold == newRule.Threshold && r.DurationInSec == newRule.DurationInSec && reflect.DeepEqual(r.SpecificItems, newRule.SpecificItems)
	if !baseCheck {
		return false
	}
	if r.ControlBehavior == Reject {
		return r.BurstCount == newRule.BurstCount
	} else if r.ControlBehavior == Throttling {
		return r.MaxQueueingTimeMs == newRule.MaxQueueingTimeMs
	} else {
		return false
	}
}

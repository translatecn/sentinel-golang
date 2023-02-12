package system

import (
	"encoding/json"
	"fmt"
)

type MetricType uint32 // 指标类型

const (
	Load           MetricType = iota // 表示Linux/Unix中的系统load1.
	AvgRT                            // 表示所有入站请求的平均响应时间.
	Concurrency                      // 并发性表示所有入站请求的并发性.
	InboundQPS                       // 表示所有入站请求的QPS.
	CpuUsage                         // 表示系统CPU占用率.
	MetricTypeSize                   // MetricType枚举大小.
)

func (t MetricType) String() string {
	switch t {
	case Load:
		return "load"
	case AvgRT:
		return "avgRT"
	case Concurrency:
		return "concurrency"
	case InboundQPS:
		return "inboundQPS"
	case CpuUsage:
		return "cpuUsage"
	default:
		return fmt.Sprintf("unknown(%d)", t)
	}
}

type AdaptiveStrategy int32

const (
	NoAdaptive AdaptiveStrategy = -1
	BBR        AdaptiveStrategy = iota // 表示基于TCP BBR思想的自适应策略.
)

func (t AdaptiveStrategy) String() string {
	switch t {
	case NoAdaptive:
		return "none"
	case BBR:
		return "bbr"
	default:
		return fmt.Sprintf("unknown(%d)", t)
	}
}

// Rule 介绍系统弹性策略.
type Rule struct {
	ID         string     `json:"id,omitempty"`
	MetricType MetricType `json:"metricType"`
	// TriggerCount表示自适应策略的下界触发器, 自适应策略将不会被激活，直到目标度量达到触发计数.
	TriggerCount float64          `json:"triggerCount"`
	Strategy     AdaptiveStrategy `json:"strategy"` // 自适应策略
}

func (r *Rule) String() string {
	b, err := json.Marshal(r)
	if err != nil {
		return fmt.Sprintf("Rule{metricType=%s, triggerCount=%.2f, adaptiveStrategy=%s}",
			r.MetricType, r.TriggerCount, r.Strategy)
	}
	return string(b)
}

func (r *Rule) ResourceName() string {
	return r.MetricType.String()
}

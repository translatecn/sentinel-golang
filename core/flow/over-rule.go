package flow

import (
	"encoding/json"
	"fmt"

	"github.com/alibaba/sentinel-golang/util"
)

// RelationStrategy 表示基于调用关系的流控制策略.
type RelationStrategy int32

const (
	CurrentResource    RelationStrategy = iota // 表示使用当前规则的resource做流控；.
	AssociatedResource                         // 表示使用关联的resource做流控，关联的resource在字段 RefResource 定义；
)

func (s RelationStrategy) String() string {
	switch s {
	case CurrentResource:
		return "CurrentResource"
	case AssociatedResource:
		return "AssociatedResource"
	default:
		return "Undefined"
	}
}

// TokenCalculateStrategy 当前流量控制器的Token计算策略.
type TokenCalculateStrategy int32

const (
	Constant       TokenCalculateStrategy = iota // Direct表示直接使用Threshold作为阈值
	WarmUp                                       // WarmUp表示使用预热方式计算Token的阈值
	MemoryAdaptive                               // MemoryAdaptive表示使用内存自适应方式计算Token的阈值
)

func (s TokenCalculateStrategy) String() string {
	switch s {
	case Constant:
		return "Constant"
	case WarmUp:
		return "WarmUp"
	case MemoryAdaptive:
		return "MemoryAdaptive"
	default:
		return "Undefined"
	}
}

// ControlBehavior // 控制行为，.
type ControlBehavior int32

const (
	Reject     ControlBehavior = iota // Reject表示直接拒绝
	Throttling                        // Throttling表示匀速排队(直到空闲容量可用为止)
)

func (s ControlBehavior) String() string {
	switch s {
	case Reject:
		return "Reject"
	case Throttling:
		return "Throttling"
	default:
		return "Undefined"
	}
}

// Rule 描述了流量控制策略，流量控制策略是基于QPS统计度量的
type Rule struct {
	ID                     string                 `json:"id,omitempty"`           // 表示规则的唯一ID(可选).
	Resource               string                 `json:"resource"`               // 表示资源名称
	TokenCalculateStrategy TokenCalculateStrategy `json:"tokenCalculateStrategy"` // 令牌计算策略
	ControlBehavior        ControlBehavior        `json:"controlBehavior"`        // 控制行为

	Threshold         float64          `json:"threshold"`         // 表示流控阈值；如果字段 StatIntervalInMs 是1000(也就是1秒)，  那么Threshold就表示QPS，流量控制器也就会依据资源的QPS来做流控.
	RelationStrategy  RelationStrategy `json:"relationStrategy"`  // 调用关联限流策略
	RefResource       string           `json:"refResource"`       // 关联资源
	MaxQueueingTimeMs uint32           `json:"maxQueueingTimeMs"` // 匀速排队的最大等待时间，该字段仅仅对控制行为是匀速排队时生效, 仅在 ControlBehavior 为 Throttling 时生效

	WarmUpPeriodSec  uint32 `json:"warmUpPeriodSec"`  // 预热的时间长度，该字段仅仅对Token计算策略是WarmUp时生效；
	WarmUpColdFactor uint32 `json:"warmUpColdFactor"` // 预热的因子，默认是3，该值的设置会影响预热的速度,该字段仅仅对Token计算策略是WarmUp时生效
	StatIntervalInMs uint32 `json:"statIntervalInMs"` // 规则对应的流量控制器的独立统计结构的统计周期.如果StatIntervalInMs是1000，也就是统计QPS.

	LowMemUsageThreshold  int64 `json:"lowMemUsageThreshold"`  // 内存低使用率时的限流阈值，该字段仅在Token计算策略是MemoryAdaptive时生效
	HighMemUsageThreshold int64 `json:"highMemUsageThreshold"` // 内存高使用率时的限流阈值，该字段仅在Token计算策略是MemoryAdaptive时生效
	MemLowWaterMarkBytes  int64 `json:"memLowWaterMarkBytes"`  // 内存低水位标记字节大小，该字段仅在Token计算策略是MemoryAdaptive时生效
	MemHighWaterMarkBytes int64 `json:"memHighWaterMarkBytes"` // 内存高水位标记字节大小，该字段仅在Token计算策略是MemoryAdaptive时生效
}

func (r *Rule) isEqualsTo(newRule *Rule) bool {
	if newRule == nil {
		return false
	}
	if !(r.Resource == newRule.Resource && r.RelationStrategy == newRule.RelationStrategy &&
		r.RefResource == newRule.RefResource && r.StatIntervalInMs == newRule.StatIntervalInMs &&
		r.TokenCalculateStrategy == newRule.TokenCalculateStrategy && r.ControlBehavior == newRule.ControlBehavior &&
		util.Float64Equals(r.Threshold, newRule.Threshold) &&
		r.MaxQueueingTimeMs == newRule.MaxQueueingTimeMs && r.WarmUpPeriodSec == newRule.WarmUpPeriodSec &&
		r.WarmUpColdFactor == newRule.WarmUpColdFactor &&
		r.LowMemUsageThreshold == newRule.LowMemUsageThreshold && r.HighMemUsageThreshold == newRule.HighMemUsageThreshold &&
		r.MemLowWaterMarkBytes == newRule.MemLowWaterMarkBytes && r.MemHighWaterMarkBytes == newRule.MemHighWaterMarkBytes) {

		return false
	}
	return true
}

func (r *Rule) isStatReusable(newRule *Rule) bool {
	if newRule == nil {
		return false
	}
	return r.Resource == newRule.Resource && r.RelationStrategy == newRule.RelationStrategy &&
		r.RefResource == newRule.RefResource && r.StatIntervalInMs == newRule.StatIntervalInMs &&
		r.needStatistic() && newRule.needStatistic()
}

// 需不需要统计指标
func (r *Rule) needStatistic() bool {
	return r.TokenCalculateStrategy == WarmUp || r.ControlBehavior == Reject
}

func (r *Rule) String() string {
	b, err := json.Marshal(r)
	if err != nil {
		// Return the fallback string
		return fmt.Sprintf("Rule{Resource=%s, TokenCalculateStrategy=%s, ControlBehavior=%s, "+
			"Threshold=%.2f, RelationStrategy=%s, RefResource=%s, MaxQueueingTimeMs=%d, WarmUpPeriodSec=%d, WarmUpColdFactor=%d, StatIntervalInMs=%d, "+
			"LowMemUsageThreshold=%v, HighMemUsageThreshold=%v, MemLowWaterMarkBytes=%v, MemHighWaterMarkBytes=%v}",
			r.Resource, r.TokenCalculateStrategy, r.ControlBehavior, r.Threshold, r.RelationStrategy, r.RefResource,
			r.MaxQueueingTimeMs, r.WarmUpPeriodSec, r.WarmUpColdFactor, r.StatIntervalInMs,
			r.LowMemUsageThreshold, r.HighMemUsageThreshold, r.MemLowWaterMarkBytes, r.MemHighWaterMarkBytes)
	}
	return string(b)
}

func (r *Rule) ResourceName() string {
	return r.Resource
}

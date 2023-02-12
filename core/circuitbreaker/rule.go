package circuitbreaker

import (
	"fmt"

	"github.com/alibaba/sentinel-golang/util"
)

type Strategy uint32

const (
	SlowRequestRatio Strategy = iota // 策略根据慢请求比改变断路器状态
	ErrorRatio
	ErrorCount
)

func (s Strategy) String() string {
	switch s {
	case SlowRequestRatio:
		return "SlowRequestRatio"
	case ErrorRatio:
		return "ErrorRatio"
	case ErrorCount:
		return "ErrorCount"
	default:
		return "Undefined"
	}
}

// Rule encompasses the fields of circuit breaking rule.
type Rule struct {
	// unique id
	Id string `json:"id,omitempty"`
	// resource name
	Resource                     string   `json:"resource"`
	Strategy                     Strategy `json:"strategy"`
	RetryTimeoutMs               uint32   `json:"retryTimeoutMs"`               // 断路器打开前的恢复 超时时间，单位为毫秒。
	MinRequestAmount             uint64   `json:"minRequestAmount"`             // 表示最小请求数 (在活动统计时间范围内) 可以触发电路断路。
	StatIntervalMs               uint32   `json:"statIntervalMs"`               // 表示内部断路器的统计时间间隔，单位为ms。
	StatSlidingWindowBucketCount uint32   `json:"statSlidingWindowBucketCount"` // 桶的个数
	MaxAllowedRtMs               uint64   `json:"maxAllowedRtMs"`               // 任何响应时间超过该值的调用(以毫秒为单位) 将被记录为慢速请求。
	// for SlowRequestRatio, it represents the max slow request ratio
	// for ErrorRatio, it represents the max error request ratio
	// for ErrorCount, it represents the max error request count
	Threshold float64 `json:"threshold"`
	ProbeNum  uint64  `json:"probeNum"` // 探测数量
}

func (r *Rule) String() string {
	// fallback string
	return fmt.Sprintf("{id=%s, resource=%s, strategy=%s, RetryTimeoutMs=%d, MinRequestAmount=%d, StatIntervalMs=%d, StatSlidingWindowBucketCount=%d, MaxAllowedRtMs=%d, Threshold=%f}",
		r.Id, r.Resource, r.Strategy, r.RetryTimeoutMs, r.MinRequestAmount, r.StatIntervalMs, r.StatSlidingWindowBucketCount, r.MaxAllowedRtMs, r.Threshold)
}

func (r *Rule) isStatReusable(newRule *Rule) bool {
	if newRule == nil {
		return false
	}
	return r.Resource == newRule.Resource && r.Strategy == newRule.Strategy && r.StatIntervalMs == newRule.StatIntervalMs &&
		r.StatSlidingWindowBucketCount == newRule.StatSlidingWindowBucketCount
}

func (r *Rule) ResourceName() string {
	return r.Resource
}

func (r *Rule) isEqualsToBase(newRule *Rule) bool {
	if newRule == nil {
		return false
	}
	return r.Resource == newRule.Resource && r.Strategy == newRule.Strategy && r.RetryTimeoutMs == newRule.RetryTimeoutMs &&
		r.MinRequestAmount == newRule.MinRequestAmount && r.StatIntervalMs == newRule.StatIntervalMs && r.StatSlidingWindowBucketCount == newRule.StatSlidingWindowBucketCount
}

func (r *Rule) isEqualsTo(newRule *Rule) bool {
	if !r.isEqualsToBase(newRule) {
		return false
	}

	switch newRule.Strategy {
	case SlowRequestRatio:
		return r.MaxAllowedRtMs == newRule.MaxAllowedRtMs && util.Float64Equals(r.Threshold, newRule.Threshold)
	case ErrorRatio:
		return util.Float64Equals(r.Threshold, newRule.Threshold)
	case ErrorCount:
		return util.Float64Equals(r.Threshold, newRule.Threshold)
	default:
		return false
	}
}

func getRuleStatSlidingWindowBucketCount(r *Rule) uint32 {
	interval := r.StatIntervalMs
	bucketCount := r.StatSlidingWindowBucketCount
	if bucketCount == 0 || interval%bucketCount != 0 {
		bucketCount = 1
	}
	return bucketCount
}

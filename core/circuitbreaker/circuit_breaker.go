package circuitbreaker

import (
	"sync/atomic"

	"github.com/alibaba/sentinel-golang/core/base"
	metric_exporter "github.com/alibaba/sentinel-golang/exporter/metric"
	"github.com/alibaba/sentinel-golang/logging"
	"github.com/alibaba/sentinel-golang/util"
	"github.com/pkg/errors"
)

//	 Circuit Breaker State Machine:
//
//	                                switch to open based on rule
//					+-----------------------------------------------------------------------+
//					|                                                                       |
//					|                                                                       v
//			+----------------+                   +----------------+      探针      +----------------+
//			|                |                   |                |<----------------|                |
//			|                |   探针 succeed     |                |                 |                |
//			|     Closed     |<------------------|    HalfOpen    |                 |      Open      |
//			|                |                   |                |   探针 failed  |                |
//			|                |                   |                +---------------->|                |
//			+----------------+                   +----------------+                 +----------------+
type State int32

const (
	Closed State = iota
	HalfOpen
	Open
)

var (
	stateChangedCounter = metric_exporter.NewCounter(
		"circuit_breaker_state_changed_total",
		"Circuit breaker total state change count",
		[]string{"resource", "from_state", "to_state"})
)

func init() {
	metric_exporter.Register(stateChangedCounter)
}

func newState() *State {
	var state State
	state = Closed

	return &state
}

func (s *State) String() string {
	switch s.get() {
	case Closed:
		return "Closed"
	case HalfOpen:
		return "HalfOpen"
	case Open:
		return "Open"
	default:
		return "Undefined"
	}
}

func (s *State) get() State {
	return State(atomic.LoadInt32((*int32)(s)))
}

func (s *State) set(update State) {
	atomic.StoreInt32((*int32)(s), int32(update))
}

func (s *State) cas(expect State, update State) bool {
	return atomic.CompareAndSwapInt32((*int32)(s), int32(expect), int32(update))
}

type StateChangeListener interface {
	OnTransformToClosed(prev State, rule Rule)                     // 关闭
	OnTransformToOpen(prev State, rule Rule, snapshot interface{}) // 打开了
	OnTransformToHalfOpen(prev State, rule Rule)                   // -> 半开
}

type CircuitBreaker interface {
	BoundRule() *Rule                    // 返回相关的断路规则。
	BoundStat() interface{}              // 返回相关的统计数据结构。
	TryPass(ctx *base.EntryContext) bool // 仅当调用时调用可用时才获得该调用的权限。
	CurrentState() State                 // 返回断路器的当前状态。
	// OnRequestComplete 记录一个已完成的请求，具有给定的响应时间和错误(如果存在)，
	// 和断路器的手柄状态转换。
	// OnRequestComplete只在传递的调用结束时调用。
	OnRequestComplete(rtt uint64, err error)
}

// ================================= circuitBreakerBase ====================================
// circuitBreakerBase encompasses the common fields of circuit breaker.
type circuitBreakerBase struct {
	rule *Rule
	// retryTimeoutMs represents recovery timeout (in milliseconds) before the circuit breaker opens.
	// During the open period, no requests are permitted until the timeout has elapsed.
	// After that, the circuit breaker will transform to half-open state for trying a few "trial" requests.
	retryTimeoutMs       uint32
	nextRetryTimestampMs uint64 // 下一次进行探测的时间
	probeNumber          uint64 // 当断路器半开时允许通过的探测请求数。
	curProbeNumber       uint64 // 当前探测数量
	state                *State // 当前断路器的数量
}

func (b *circuitBreakerBase) BoundRule() *Rule {
	return b.rule
}

func (b *circuitBreakerBase) CurrentState() State {
	return b.state.get()
}

func (b *circuitBreakerBase) retryTimeoutArrived() bool {
	return util.CurrentTimeMillis() >= atomic.LoadUint64(&b.nextRetryTimestampMs)
}

func (b *circuitBreakerBase) updateNextRetryTimestamp() {
	atomic.StoreUint64(&b.nextRetryTimestampMs, util.CurrentTimeMillis()+uint64(b.retryTimeoutMs))
}

// 添加当前探测数量
func (b *circuitBreakerBase) addCurProbeNum() {
	atomic.AddUint64(&b.curProbeNumber, 1)
}

// 重置当前探测数量
func (b *circuitBreakerBase) resetCurProbeNum() {
	atomic.StoreUint64(&b.curProbeNumber, 0)
}

// fromClosedToOpen 更新断路器状态机由闭合状态变为开启状态。
// 仅当当前goroutine成功完成转换时返回true。
func (b *circuitBreakerBase) fromClosedToOpen(snapshot interface{}) bool {
	if b.state.cas(Closed, Open) {
		b.updateNextRetryTimestamp()
		for _, listener := range stateChangeListeners {
			listener.OnTransformToOpen(Closed, *b.rule, snapshot)
		}
		stateChangedCounter.Add(float64(1), b.BoundRule().Resource, "Closed", "Open")
		return true
	}
	return false
}

// fromOpenToHalfOpen 将断路器状态机从开到半开。
func (b *circuitBreakerBase) fromOpenToHalfOpen(ctx *base.EntryContext) bool {
	if b.state.cas(Open, HalfOpen) {
		for _, listener := range stateChangeListeners {
			listener.OnTransformToHalfOpen(Open, *b.rule)
		}

		entry := ctx.Entry()
		if entry == nil {
			logging.Error(errors.New("nil entry"), "Nil entry in circuitBreakerBase.fromOpenToHalfOpen()", "rule", b.rule)
		} else {
			// add hook for entry exit
			// if the current circuit breaker performs the probe through this entry, but the entry was blocked,
			// this hook will guarantee current circuit breaker state machine will rollback to Open from Half-Open
			entry.WhenExit(func(entry *base.SentinelEntry, ctx *base.EntryContext) error {
				if ctx.IsBlocked() && b.state.cas(HalfOpen, Open) {
					for _, listener := range stateChangeListeners {
						listener.OnTransformToOpen(HalfOpen, *b.rule, 1.0)
					}
				}
				return nil
			})
		}

		stateChangedCounter.Add(float64(1), b.BoundRule().Resource, "Open", "HalfOpen")
		return true
	}
	return false
}

// fromHalfOpenToOpen 将断路器状态机从半开状态更新为开状态。
// 仅当当前goroutine成功完成转换时返回true。
func (b *circuitBreakerBase) fromHalfOpenToOpen(snapshot interface{}) bool {
	if b.state.cas(HalfOpen, Open) {
		b.resetCurProbeNum()
		b.updateNextRetryTimestamp()
		for _, listener := range stateChangeListeners {
			listener.OnTransformToOpen(HalfOpen, *b.rule, snapshot)
		}
		stateChangedCounter.Add(float64(1), b.BoundRule().Resource, "HalfOpen", "Open")
		return true
	}
	return false
}

// fromHalfOpenToOpen 将断路器状态机从半开状态更新为闭合状态
// 仅当当前goroutine成功完成转换时返回true。
func (b *circuitBreakerBase) fromHalfOpenToClosed() bool {
	if b.state.cas(HalfOpen, Closed) {
		b.resetCurProbeNum()
		for _, listener := range stateChangeListeners { // 触发所有监听者
			listener.OnTransformToClosed(HalfOpen, *b.rule)
		}

		stateChangedCounter.Add(float64(1), b.BoundRule().Resource, "HalfOpen", "Closed")
		return true
	}
	return false
}

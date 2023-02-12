package circuitbreaker

import (
	"github.com/alibaba/sentinel-golang/core/base"
	sbase "github.com/alibaba/sentinel-golang/core/stat/base"
	"github.com/alibaba/sentinel-golang/logging"
	"sync/atomic"
)

// ================================= errorCountCircuitBreaker ====================================
type errorCountCircuitBreaker struct {
	circuitBreakerBase
	minRequestAmount    uint64
	errorCountThreshold uint64

	stat *errorCounterLeapArray
}

func newErrorCountCircuitBreakerWithStat(r *Rule, stat *errorCounterLeapArray) *errorCountCircuitBreaker {
	return &errorCountCircuitBreaker{
		circuitBreakerBase: circuitBreakerBase{
			rule:                 r,
			retryTimeoutMs:       r.RetryTimeoutMs,
			nextRetryTimestampMs: 0,
			state:                newState(),
			probeNumber:          r.ProbeNum,
		},
		minRequestAmount:    r.MinRequestAmount,
		errorCountThreshold: uint64(r.Threshold),
		stat:                stat,
	}
}

func newErrorCountCircuitBreaker(r *Rule) (*errorCountCircuitBreaker, error) {
	interval := r.StatIntervalMs
	bucketCount := getRuleStatSlidingWindowBucketCount(r)
	stat := &errorCounterLeapArray{}
	leapArray, err := sbase.NewLeapArray(bucketCount, interval, stat)
	if err != nil {
		return nil, err
	}
	stat.data = leapArray
	return newErrorCountCircuitBreakerWithStat(r, stat), nil
}

func (b *errorCountCircuitBreaker) BoundStat() interface{} {
	return b.stat
}

func (b *errorCountCircuitBreaker) TryPass(ctx *base.EntryContext) bool {
	curStatus := b.CurrentState()
	if curStatus == Closed {
		return true
	} else if curStatus == Open {
		if b.retryTimeoutArrived() && b.fromOpenToHalfOpen(ctx) {
			// 到达下一次探测的时间了,
			return true
		}
	} else if curStatus == HalfOpen && b.probeNumber > 0 {
		return true
	}
	return false
}

// OnRequestComplete 会更新对应的状态
func (b *errorCountCircuitBreaker) OnRequestComplete(_ uint64, err error) {
	metricStat := b.stat
	counter, curErr := metricStat.currentCounter() // 当前时间所对应的bucket的计数
	if curErr != nil {
		logging.Error(curErr, "Fail to get current counter in errorCountCircuitBreaker#OnRequestComplete().",
			"rule", b.rule)
		return
	}
	if err != nil {
		atomic.AddUint64(&counter.errorCount, 1)
	}
	atomic.AddUint64(&counter.totalCount, 1)

	errorCount := uint64(0)
	totalCount := uint64(0)
	counters := metricStat.allCounter() // 获取采集时间内的所有错误数,以及总请求数
	for _, c := range counters {
		errorCount += atomic.LoadUint64(&c.errorCount)
		totalCount += atomic.LoadUint64(&c.totalCount)
	}
	// handleStateChangeWhenThresholdExceeded
	curStatus := b.CurrentState()
	if curStatus == Open {
		return
	}
	if curStatus == HalfOpen {
		if err == nil {
			b.addCurProbeNum() // 添加当前探测数量
			if b.probeNumber == 0 || atomic.LoadUint64(&b.curProbeNumber) >= b.probeNumber {
				b.fromHalfOpenToClosed() // 将半开-> close
				b.resetMetric()          // 重置监听时间内的所有指标
			}
		} else {
			b.fromHalfOpenToOpen(1)
		}
		return
	}

	if totalCount < b.minRequestAmount {
		return
	}
	if errorCount >= b.errorCountThreshold {
		curStatus = b.CurrentState()
		switch curStatus {
		case Closed:
			b.fromClosedToOpen(errorCount)
		case HalfOpen:
			b.fromHalfOpenToOpen(errorCount)
		default:
		}
	}
}

func (b *errorCountCircuitBreaker) resetMetric() {
	for _, c := range b.stat.allCounter() {
		c.reset()
	}
}

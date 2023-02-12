package circuitbreaker

import (
	"github.com/alibaba/sentinel-golang/core/base"
	sbase "github.com/alibaba/sentinel-golang/core/stat/base"
	"github.com/alibaba/sentinel-golang/logging"
	"github.com/alibaba/sentinel-golang/util"
	"github.com/pkg/errors"
	"reflect"
	"sync/atomic"
)

// ================================= slowRtCircuitBreaker ====================================
type slowRtCircuitBreaker struct {
	circuitBreakerBase
	stat                *slowRequestLeapArray
	maxAllowedRt        uint64
	maxSlowRequestRatio float64
	minRequestAmount    uint64
}

func newSlowRtCircuitBreakerWithStat(r *Rule, stat *slowRequestLeapArray) *slowRtCircuitBreaker {
	return &slowRtCircuitBreaker{
		circuitBreakerBase: circuitBreakerBase{
			rule:                 r,
			retryTimeoutMs:       r.RetryTimeoutMs,
			nextRetryTimestampMs: 0,
			state:                newState(),
			probeNumber:          r.ProbeNum,
		},
		stat:                stat,
		maxAllowedRt:        r.MaxAllowedRtMs,
		maxSlowRequestRatio: r.Threshold,
		minRequestAmount:    r.MinRequestAmount,
	}
}

func newSlowRtCircuitBreaker(r *Rule) (*slowRtCircuitBreaker, error) {
	interval := r.StatIntervalMs
	bucketCount := getRuleStatSlidingWindowBucketCount(r)
	stat := &slowRequestLeapArray{}
	leapArray, err := sbase.NewLeapArray(bucketCount, interval, stat)
	if err != nil {
		return nil, err
	}
	stat.data = leapArray

	return newSlowRtCircuitBreakerWithStat(r, stat), nil
}

func (b *slowRtCircuitBreaker) BoundStat() interface{} {
	return b.stat
}

// TryPass checks circuit breaker based on state machine of circuit breaker.
func (b *slowRtCircuitBreaker) TryPass(ctx *base.EntryContext) bool {
	curStatus := b.CurrentState()
	if curStatus == Closed {
		return true
	} else if curStatus == Open {
		// switch state to half-open to probe if retry timeout
		if b.retryTimeoutArrived() && b.fromOpenToHalfOpen(ctx) {
			return true
		}
	} else if curStatus == HalfOpen && b.probeNumber > 0 {
		return true
	}
	return false
}

func (b *slowRtCircuitBreaker) OnRequestComplete(rt uint64, _ error) {
	// add slow and add total
	metricStat := b.stat
	counter, curErr := metricStat.currentCounter()
	if curErr != nil {
		logging.Error(curErr, "Fail to get current counter in slowRtCircuitBreaker#OnRequestComplete().",
			"rule", b.rule)
		return
	}
	if rt > b.maxAllowedRt {
		atomic.AddUint64(&counter.slowCount, 1)
	}
	atomic.AddUint64(&counter.totalCount, 1)

	slowCount := uint64(0)
	totalCount := uint64(0)
	counters := metricStat.allCounter()
	for _, c := range counters {
		slowCount += atomic.LoadUint64(&c.slowCount)
		totalCount += atomic.LoadUint64(&c.totalCount)
	}
	slowRatio := float64(slowCount) / float64(totalCount)

	// handleStateChange
	curStatus := b.CurrentState()
	if curStatus == Open {
		return
	} else if curStatus == HalfOpen {
		if rt > b.maxAllowedRt {
			// fail to probe
			b.fromHalfOpenToOpen(1.0)
		} else {
			b.addCurProbeNum()
			if b.probeNumber == 0 || atomic.LoadUint64(&b.curProbeNumber) >= b.probeNumber {
				// succeed to probe
				b.fromHalfOpenToClosed()
				b.resetMetric()
			}
		}
		return
	}

	// current state is CLOSED
	if totalCount < b.minRequestAmount {
		return
	}

	if slowRatio > b.maxSlowRequestRatio || util.Float64Equals(slowRatio, b.maxSlowRequestRatio) {
		curStatus = b.CurrentState()
		switch curStatus {
		case Closed:
			b.fromClosedToOpen(slowRatio)
		case HalfOpen:
			b.fromHalfOpenToOpen(slowRatio)
		default:
		}
	}
	return
}

func (b *slowRtCircuitBreaker) resetMetric() {
	for _, c := range b.stat.allCounter() {
		c.reset()
	}
}

type slowRequestCounter struct {
	slowCount  uint64
	totalCount uint64
}

func (c *slowRequestCounter) reset() {
	atomic.StoreUint64(&c.slowCount, 0)
	atomic.StoreUint64(&c.totalCount, 0)
}

type slowRequestLeapArray struct {
	data *sbase.LeapArray
}

func (s *slowRequestLeapArray) NewEmptyBucket() interface{} {
	return &slowRequestCounter{
		slowCount:  0,
		totalCount: 0,
	}
}

func (s *slowRequestLeapArray) ResetBucketTo(bw *sbase.BucketWrap, startTime uint64) *sbase.BucketWrap {
	atomic.StoreUint64(&bw.BucketStart, startTime)
	bw.Value.Store(&slowRequestCounter{
		slowCount:  0,
		totalCount: 0,
	})
	return bw
}

func (s *slowRequestLeapArray) currentCounter() (*slowRequestCounter, error) {
	curBucket, err := s.data.CurrentBucket(s)
	if err != nil {
		return nil, err
	}
	if curBucket == nil {
		return nil, errors.New("nil BucketWrap")
	}
	mb := curBucket.Value.Load()
	if mb == nil {
		return nil, errors.New("nil slowRequestCounter")
	}
	counter, ok := mb.(*slowRequestCounter)
	if !ok {
		return nil, errors.Errorf("bucket fail to do type assert, expect: *slowRequestCounter, in fact: %s", reflect.TypeOf(mb).Name())
	}
	return counter, nil
}

func (s *slowRequestLeapArray) allCounter() []*slowRequestCounter {
	buckets := s.data.Values()
	ret := make([]*slowRequestCounter, 0, len(buckets))
	for _, b := range buckets {
		mb := b.Value.Load()
		if mb == nil {
			logging.Error(errors.New("current bucket atomic Value is nil"), "Current bucket atomic Value is nil in slowRequestLeapArray.allCounter()")
			continue
		}
		counter, ok := mb.(*slowRequestCounter)
		if !ok {
			logging.Error(errors.New("bucket data type error"), "Bucket data type error in slowRequestLeapArray.allCounter()", "expect type", "*slowRequestCounter", "actual type", reflect.TypeOf(mb).Name())
			continue
		}
		ret = append(ret, counter)
	}
	return ret
}

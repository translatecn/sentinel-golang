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

// ================================= errorRatioCircuitBreaker ====================================
type errorRatioCircuitBreaker struct {
	circuitBreakerBase
	minRequestAmount    uint64
	errorRatioThreshold float64

	stat *errorCounterLeapArray
}

func newErrorRatioCircuitBreakerWithStat(r *Rule, stat *errorCounterLeapArray) *errorRatioCircuitBreaker {
	return &errorRatioCircuitBreaker{
		circuitBreakerBase: circuitBreakerBase{
			rule:                 r,
			retryTimeoutMs:       r.RetryTimeoutMs,
			nextRetryTimestampMs: 0,
			state:                newState(),
			probeNumber:          r.ProbeNum,
		},
		minRequestAmount:    r.MinRequestAmount,
		errorRatioThreshold: r.Threshold,
		stat:                stat,
	}
}

func newErrorRatioCircuitBreaker(r *Rule) (*errorRatioCircuitBreaker, error) {
	interval := r.StatIntervalMs
	bucketCount := getRuleStatSlidingWindowBucketCount(r)
	stat := &errorCounterLeapArray{}
	leapArray, err := sbase.NewLeapArray(bucketCount, interval, stat)
	if err != nil {
		return nil, err
	}
	stat.data = leapArray
	return newErrorRatioCircuitBreakerWithStat(r, stat), nil
}

func (b *errorRatioCircuitBreaker) BoundStat() interface{} {
	return b.stat
}

func (b *errorRatioCircuitBreaker) TryPass(ctx *base.EntryContext) bool {
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

func (b *errorRatioCircuitBreaker) OnRequestComplete(_ uint64, err error) {
	metricStat := b.stat
	counter, curErr := metricStat.currentCounter()
	if curErr != nil {
		logging.Error(curErr, "Fail to get current counter in errorRatioCircuitBreaker#OnRequestComplete().",
			"rule", b.rule)
		return
	}
	if err != nil {
		atomic.AddUint64(&counter.errorCount, 1)
	}
	atomic.AddUint64(&counter.totalCount, 1)

	errorCount := uint64(0)
	totalCount := uint64(0)
	counters := metricStat.allCounter()
	for _, c := range counters {
		errorCount += atomic.LoadUint64(&c.errorCount)
		totalCount += atomic.LoadUint64(&c.totalCount)
	}
	errorRatio := float64(errorCount) / float64(totalCount)

	// handleStateChangeWhenThresholdExceeded
	curStatus := b.CurrentState()
	if curStatus == Open {
		return
	}
	if curStatus == HalfOpen {
		if err == nil {
			b.addCurProbeNum()
			if b.probeNumber == 0 || atomic.LoadUint64(&b.curProbeNumber) >= b.probeNumber {
				b.fromHalfOpenToClosed()
				b.resetMetric()
			}
		} else {
			b.fromHalfOpenToOpen(1.0)
		}
		return
	}

	// current state is CLOSED
	if totalCount < b.minRequestAmount {
		return
	}
	if errorRatio > b.errorRatioThreshold || util.Float64Equals(errorRatio, b.errorRatioThreshold) {
		curStatus = b.CurrentState()
		switch curStatus {
		case Closed:
			b.fromClosedToOpen(errorRatio)
		case HalfOpen:
			b.fromHalfOpenToOpen(errorRatio)
		default:
		}
	}
}

func (b *errorRatioCircuitBreaker) resetMetric() {
	for _, c := range b.stat.allCounter() {
		c.reset()
	}
}

type errorCounter struct {
	errorCount uint64
	totalCount uint64
}

func (c *errorCounter) reset() {
	atomic.StoreUint64(&c.errorCount, 0)
	atomic.StoreUint64(&c.totalCount, 0)
}

type errorCounterLeapArray struct {
	data *sbase.LeapArray
}

func (s *errorCounterLeapArray) NewEmptyBucket() interface{} {
	return &errorCounter{
		errorCount: 0,
		totalCount: 0,
	}
}

func (s *errorCounterLeapArray) ResetBucketTo(bw *sbase.BucketWrap, startTime uint64) *sbase.BucketWrap {
	atomic.StoreUint64(&bw.BucketStart, startTime)
	bw.Value.Store(&errorCounter{
		errorCount: 0,
		totalCount: 0,
	})
	return bw
}

// 断路器当前计数
func (s *errorCounterLeapArray) currentCounter() (*errorCounter, error) {
	curBucket, err := s.data.CurrentBucket(s)
	if err != nil {
		return nil, err
	}
	if curBucket == nil {
		return nil, errors.New("nil BucketWrap")
	}
	mb := curBucket.Value.Load()
	if mb == nil {
		return nil, errors.New("nil errorCounter")
	}
	counter, ok := mb.(*errorCounter)
	if !ok {
		return nil, errors.Errorf("bucket fail to do type assert, expect: *errorCounter, in fact: %s", reflect.TypeOf(mb).Name())
	}
	return counter, nil
}

func (s *errorCounterLeapArray) allCounter() []*errorCounter {
	buckets := s.data.Values()
	ret := make([]*errorCounter, 0, len(buckets))
	for _, b := range buckets {
		mb := b.Value.Load()
		if mb == nil {
			logging.Error(errors.New("current bucket atomic Value is nil"), "Current bucket atomic Value is nil in errorCounterLeapArray.allCounter()")
			continue
		}
		counter, ok := mb.(*errorCounter)
		if !ok {
			logging.Error(errors.New("bucket data type error"), "Bucket data type error in errorCounterLeapArray.allCounter()", "expect type", "*errorCounter", "actual type", reflect.TypeOf(mb).Name())
			continue
		}
		ret = append(ret, counter)
	}
	return ret
}

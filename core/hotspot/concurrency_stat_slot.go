package hotspot

import (
	"sync/atomic"

	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/logging"
)

const (
	StatSlotOrder = 4000
)

var (
	DefaultConcurrencyStatSlot = &ConcurrencyStatSlot{}
)

type ConcurrencyStatSlot struct {
}

func (s *ConcurrencyStatSlot) Order() uint32 {
	return StatSlotOrder
}

func (c *ConcurrencyStatSlot) OnEntryPassed(ctx *base.EntryContext) {
	res := ctx.Resource.Name()
	tcs := getTrafficControllersFor(res)
	for _, tc := range tcs {
		if tc.BoundRule().MetricType != Concurrency {
			continue
		}
		arg := tc.ExtractArgs(ctx)
		if arg == nil {
			continue
		}
		metric := tc.BoundMetric()
		concurrencyPtr, existed := metric.ConcurrencyCounter.Get(arg)
		if !existed || concurrencyPtr == nil {
			if logging.DebugEnabled() {
				logging.Debug("[ConcurrencyStatSlot OnEntryPassed] Parameter does not exist in ConcurrencyCounter.", "argument", arg)
			}
			continue
		}
		atomic.AddInt64(concurrencyPtr, 1)
	}
}

func (c *ConcurrencyStatSlot) OnEntryBlocked(ctx *base.EntryContext, blockError *base.BlockError) {
	// Do nothing
}

func (c *ConcurrencyStatSlot) OnCompleted(ctx *base.EntryContext) { // 并发计数
	res := ctx.Resource.Name()
	tcs := getTrafficControllersFor(res)
	for _, tc := range tcs {
		if tc.BoundRule().MetricType != Concurrency {
			continue
		}
		arg := tc.ExtractArgs(ctx)
		if arg == nil {
			continue
		}
		metric := tc.BoundMetric()
		concurrencyPtr, existed := metric.ConcurrencyCounter.Get(arg)
		if !existed || concurrencyPtr == nil {
			if logging.DebugEnabled() {
				logging.Debug("[ConcurrencyStatSlot OnCompleted] Parameter does not exist in ConcurrencyCounter.", "argument", arg)
			}
			continue
		}
		atomic.AddInt64(concurrencyPtr, -1)
	}
}

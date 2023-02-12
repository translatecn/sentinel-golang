package circuitbreaker

import (
	"github.com/alibaba/sentinel-golang/core/base"
)

const (
	StatSlotOrder = 5000
)

var (
	DefaultMetricStatSlot = &MetricStatSlot{}
)

// MetricStatSlot 记录断路器调用完成时的度量。
// 如果断路器活，则必须将MetricStatSlot填充到槽链中。
type MetricStatSlot struct {
}

func (s *MetricStatSlot) Order() uint32 {
	return StatSlotOrder
}

func (c *MetricStatSlot) OnEntryPassed(_ *base.EntryContext) {
	return
}

func (c *MetricStatSlot) OnEntryBlocked(_ *base.EntryContext, _ *base.BlockError) {
	return
}

func (c *MetricStatSlot) OnCompleted(ctx *base.EntryContext) {
	res := ctx.Resource.Name()
	err := ctx.Err()
	rt := ctx.Rt()
	for _, cb := range getBreakersOfResource(res) {
		cb.OnRequestComplete(rt, err)
	}
}

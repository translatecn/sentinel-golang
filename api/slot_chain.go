package api

import (
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/core/circuitbreaker"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/alibaba/sentinel-golang/core/hotspot"
	"github.com/alibaba/sentinel-golang/core/isolation"
	"github.com/alibaba/sentinel-golang/core/log"
	"github.com/alibaba/sentinel-golang/core/stat"
	"github.com/alibaba/sentinel-golang/core/system"
)

var globalSlotChain = BuildDefaultSlotChain()

func GlobalSlotChain() *base.SlotChain {
	return globalSlotChain
}

func BuildDefaultSlotChain() *base.SlotChain {
	sc := base.NewSlotChain()
	sc.AddStatPrepareSlot(stat.DefaultResourceNodePrepareSlot)

	sc.AddRuleCheckSlot(system.DefaultAdaptiveSlot)
	sc.AddRuleCheckSlot(flow.DefaultSlot)
	sc.AddRuleCheckSlot(isolation.DefaultSlot)
	sc.AddRuleCheckSlot(hotspot.DefaultSlot)
	sc.AddRuleCheckSlot(circuitbreaker.DefaultSlot)

	sc.AddStatSlot(stat.DefaultSlot)
	sc.AddStatSlot(log.DefaultSlot)
	sc.AddStatSlot(flow.DefaultStandaloneStatSlot)
	sc.AddStatSlot(hotspot.DefaultConcurrencyStatSlot)
	sc.AddStatSlot(circuitbreaker.DefaultMetricStatSlot)
	return sc
}

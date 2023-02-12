package api

import (
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/core/circuitbreaker"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/alibaba/sentinel-golang/core/hotspot"
	"github.com/alibaba/sentinel-golang/core/isolation"
	"github.com/alibaba/sentinel-golang/core/stat"
	"github.com/alibaba/sentinel-golang/core/system"
)

var globalSlotChain = BuildDefaultSlotChain()

func GlobalSlotChain() *base.SlotChain {
	return globalSlotChain
}

// BuildDefaultSlotChain 构建默认的槽链表
func BuildDefaultSlotChain() *base.SlotChain {
	sc := base.NewSlotChain()
	sc.AddStatPrepareSlot(stat.DefaultResourceNodePrepareSlot)

	sc.AddRuleCheckSlot(system.DefaultAdaptiveSlot)
	sc.AddRuleCheckSlot(flow.DefaultSlot)           // 流量控制
	sc.AddRuleCheckSlot(isolation.DefaultSlot)      // 并发控制
	sc.AddRuleCheckSlot(hotspot.DefaultSlot)        // 热点
	sc.AddRuleCheckSlot(circuitbreaker.DefaultSlot) // 断路器

	sc.AddStatSlot(stat.DefaultSlot)
	sc.AddStatSlot(flow.DefaultStandaloneStatSlot)       // 流量控制
	sc.AddStatSlot(hotspot.DefaultConcurrencyStatSlot)   // 热点
	sc.AddStatSlot(circuitbreaker.DefaultMetricStatSlot) // 断路器
	return sc
}

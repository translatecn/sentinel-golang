package flow

import (
	"github.com/alibaba/sentinel-golang/core/base"
)

type DirectTrafficShapingCalculator struct {
	owner     *TrafficShapingController
	threshold float64
}

func NewDirectTrafficShapingCalculator(owner *TrafficShapingController, threshold float64) *DirectTrafficShapingCalculator {
	return &DirectTrafficShapingCalculator{
		owner:     owner,
		threshold: threshold,
	}
}

func (d *DirectTrafficShapingCalculator) CalculateAllowedTokens(uint32, int32) float64 {
	return d.threshold
}

func (d *DirectTrafficShapingCalculator) BoundOwner() *TrafficShapingController {
	return d.owner
}

type RejectTrafficShapingChecker struct {
	owner *TrafficShapingController
	rule  *Rule
}

func NewRejectTrafficShapingChecker(owner *TrafficShapingController, rule *Rule) *RejectTrafficShapingChecker {
	return &RejectTrafficShapingChecker{
		owner: owner,
		rule:  rule,
	}
}

func (d *RejectTrafficShapingChecker) BoundOwner() *TrafficShapingController {
	return d.owner
}

// DoCheck 参数中threshold则是token计算策略中计算出的限流阈值
func (d *RejectTrafficShapingChecker) DoCheck(resStat base.StatNode, batchCount uint32, threshold float64) *base.TokenResult {
	// 获取统计结构
	metricReadonlyStat := d.BoundOwner().boundStat.readOnlyMetric // 当前指标的度量值
	if metricReadonlyStat == nil {
		return nil
	}
	// 获取当前统计周期内已通过的请求数量  tc.boundStat.writeOnlyMetric.AddCount(base.MetricEventPass, int64(ctx.Input.BatchCount))
	curCount := float64(metricReadonlyStat.GetSum(base.MetricEventPass))
	// 已通过的请求+当前请求>限流阈值则直接返回限流结果
	if curCount+float64(batchCount) > threshold {
		msg := "flow reject check blocked"
		return base.NewTokenResultBlockedWithCause(base.BlockTypeFlow, msg, d.rule, curCount)
	}
	return nil
}

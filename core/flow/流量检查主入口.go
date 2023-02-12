package flow

import (
	"github.com/alibaba/sentinel-golang/core/base"
	metric_exporter "github.com/alibaba/sentinel-golang/exporter/metric"
)

var (
	resourceFlowThresholdGauge = metric_exporter.NewGauge(
		"resource_flow_threshold",
		"Resource flow threshold",
		[]string{"resource"})
)

func init() {
	metric_exporter.Register(resourceFlowThresholdGauge)
}

// TrafficShapingCalculator 根据规则阈值 和token计算策略计算实际的阈值
type TrafficShapingCalculator interface {
	BoundOwner() *TrafficShapingController
	CalculateAllowedTokens(batchCount uint32, flag int32) float64 // 返回当前状态下的令牌阈值
}

// TrafficShapingChecker 根据当前指标和控制行为进行检查，生成token控制结果.
type TrafficShapingChecker interface {
	BoundOwner() *TrafficShapingController
	DoCheck(resStat base.StatNode, batchCount uint32, threshold float64) *base.TokenResult
}

// standaloneStatistic 表示每个TrafficShapingController的独立统计量
type standaloneStatistic struct {
	reuseResourceStat bool           // 指示当前独立统计是否重用当前资源的全局统计
	readOnlyMetric    base.ReadStat  // 只读度量统计量. true，它将是重用的SlidingWindowMetric;  false，它将是BucketLeapArray
	writeOnlyMetric   base.WriteStat // 只写度量统计量. true，它将为nil  ;  false，它将是BucketLeapArray
}

// TrafficShapingController 流量控制
type TrafficShapingController struct {
	flowCalculator TrafficShapingCalculator
	flowChecker    TrafficShapingChecker
	rule           *Rule
	boundStat      standaloneStatistic // 当前指标的度量值
}

func NewTrafficShapingController(rule *Rule, boundStat *standaloneStatistic) (*TrafficShapingController, error) {
	return &TrafficShapingController{rule: rule, boundStat: *boundStat}, nil
}

func (t *TrafficShapingController) BoundRule() *Rule {
	return t.rule
}

func (t *TrafficShapingController) FlowChecker() TrafficShapingChecker {
	return t.flowChecker
}

func (t *TrafficShapingController) FlowCalculator() TrafficShapingCalculator {
	return t.flowCalculator
}

func (t *TrafficShapingController) PerformChecking(resStat base.StatNode, batchCount uint32, flag int32) *base.TokenResult {
	allowedTokens := t.flowCalculator.CalculateAllowedTokens(batchCount, flag) // 根据规则阈值 和token计算策略计算实际的阈值
	resourceFlowThresholdGauge.Set(allowedTokens, t.rule.Resource)             // 上报指标
	return t.flowChecker.DoCheck(resStat, batchCount, allowedTokens)
}

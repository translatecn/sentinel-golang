package flow

import (
	"math"
	"sync/atomic"

	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/core/config"
	"github.com/alibaba/sentinel-golang/logging"
	"github.com/alibaba/sentinel-golang/util"
)

type WarmUpTrafficShapingCalculator struct {
	owner             *TrafficShapingController
	threshold         float64
	warmUpPeriodInSec uint32
	coldFactor        uint32
	warningToken      uint64
	maxToken          uint64
	slope             float64
	storedTokens      int64
	lastFilledTime    uint64
}

func (c *WarmUpTrafficShapingCalculator) BoundOwner() *TrafficShapingController {
	return c.owner
}

func NewWarmUpTrafficShapingCalculator(owner *TrafficShapingController, rule *Rule) TrafficShapingCalculator {
	if rule.WarmUpColdFactor <= 1 {
		rule.WarmUpColdFactor = config.DefaultWarmUpColdFactor
		logging.Warn("[NewWarmUpTrafficShapingCalculator] No set WarmUpColdFactor,use default warm up cold factor value", "defaultWarmUpColdFactor", config.DefaultWarmUpColdFactor)
	}

	// 这俩参数主要由 预热周期,预热因子,周期控制

	// 令牌预警数量，即令牌桶中的剩余令牌数量到达预警值时，预热结束。[一定时间内请求的几分之几]
	warningToken := uint64((float64(rule.WarmUpPeriodSec) * rule.Threshold) / float64(rule.WarmUpColdFactor-1))

	// 令牌桶最大容量，当令牌桶到达容量后，生成的令牌将被丢弃
	maxToken := warningToken + uint64(2*float64(rule.WarmUpPeriodSec)*rule.Threshold/float64(1.0+rule.WarmUpColdFactor))

	slope := float64(rule.WarmUpColdFactor-1.0) / rule.Threshold / float64(maxToken-warningToken)

	warmUpTrafficShapingCalculator := &WarmUpTrafficShapingCalculator{
		owner:             owner,
		warmUpPeriodInSec: rule.WarmUpPeriodSec,
		coldFactor:        rule.WarmUpColdFactor,
		warningToken:      warningToken,
		maxToken:          maxToken,
		slope:             slope,
		threshold:         rule.Threshold,
		storedTokens:      0,
		lastFilledTime:    0,
	}

	return warmUpTrafficShapingCalculator
}

func (c *WarmUpTrafficShapingCalculator) CalculateAllowedTokens(_ uint32, _ int32) float64 {
	// 获取滑动时间窗口前一个统计周期的QPS，依托于底层的统计结构
	metricReadonlyStat := c.BoundOwner().boundStat.readOnlyMetric
	previousQps := metricReadonlyStat.GetPreviousQPS(base.MetricEventPass)
	// 同步令牌桶中的令牌(包括生成和丢弃)
	c.syncToken(previousQps)
	// 原子获取令牌桶中的令牌数
	restToken := atomic.LoadInt64(&c.storedTokens)
	if restToken < 0 {
		restToken = 0
	}
	// 如果桶中令牌数>=令牌预警线(500)，代表还在冷启动阶段
	if restToken >= int64(c.warningToken) {
		// 计算桶中令牌和预警线的差值(也就是还有多少个令牌可用)
		aboveToken := restToken - int64(c.warningToken)
		// 动态计算出每秒允许通过的QPS阈值
		warningQps := math.Nextafter(1.0/(float64(aboveToken)*c.slope+1.0/c.threshold), math.MaxFloat64)
		return warningQps
	} else {
		// 如果桶中令牌数<令牌预警线，则说明冷启动已经结束，直接返回规则中的阈值
		return c.threshold
	}
}

// 更新令牌桶中令牌的数量
func (c *WarmUpTrafficShapingCalculator) syncToken(passQps float64) {
	// 获取当前时间(毫秒)
	currentTime := util.CurrentTimeMillis()
	// 获取当前时间(秒)
	currentTime = currentTime - currentTime%1000

	// 最后填充令牌桶时间
	oldLastFillTime := atomic.LoadUint64(&c.lastFilledTime)
	// 如果当前时间小于最后填充时间，说明出现了时间回拨，则不需要填充/丢弃令牌
	// 如果当前时间等于最后填充时间，说明在同一秒内已经同步过令牌桶了，避免重复填充/丢弃令牌
	if currentTime <= oldLastFillTime {
		return
	}

	// 获取当前桶中的令牌数量
	oldValue := atomic.LoadInt64(&c.storedTokens)
	// 初始化/生成令牌
	newValue := c.coolDownTokens(currentTime, passQps)
	// 利用cas存储最新的令牌数量，避免并发不安全问题。
	if atomic.CompareAndSwapInt64(&c.storedTokens, oldValue, newValue) {
		// 最终桶中令牌数=桶中令牌数-已经通过的QPS
		if currentValue := atomic.AddInt64(&c.storedTokens, int64(-passQps)); currentValue < 0 {
			atomic.StoreInt64(&c.storedTokens, 0)
		}
		// 更新最后更新令牌桶的时间
		atomic.StoreUint64(&c.lastFilledTime, currentTime)
	}
}

// 初始化令牌桶以及填充令牌：
func (c *WarmUpTrafficShapingCalculator) coolDownTokens(currentTime uint64, passQps float64) int64 {
	oldValue := atomic.LoadInt64(&c.storedTokens)
	newValue := oldValue

	// 如果令牌桶中的令牌数量小于令牌预警线
	// 初始化时桶中令牌=0一定小于warningToken
	// 预热结束后，令牌桶中的数量也会小于预警线
	if oldValue < int64(c.warningToken) {
		// 填充令牌=桶中令牌数+每秒应该生成的令牌数100
		newValue = int64(float64(oldValue) + (float64(currentTime)-float64(atomic.LoadUint64(&c.lastFilledTime)))*c.threshold/1000.0)
	} else if oldValue > int64(c.warningToken) {
		// 如果令牌数量大于预警线，说明应该在预热期间
		// 但是如果通过的请求数(消费的令牌数)小于冷却数量，说明并没有真正的开始预热
		// 则需要填充令牌，让桶中令牌维持在maxToken
		if passQps < float64(uint32(c.threshold)/c.coldFactor) {
			newValue = int64(float64(oldValue) + float64(currentTime-atomic.LoadUint64(&c.lastFilledTime))*c.threshold/1000.0)
		}
	}
	// 当前生成的令牌小于最大令牌数
	if newValue <= int64(c.maxToken) {
		return newValue
	} else {
		// 如果但前令牌大雨最大令牌，则丢弃多余令牌，让桶中始终最多拥有maxToken
		return int64(c.maxToken)
	}
}

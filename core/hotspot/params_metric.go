package hotspot

import "github.com/alibaba/sentinel-golang/core/hotspot/cache"

const (
	ConcurrencyMaxCount = 4000
	ParamsCapacityBase  = 4000
	ParamsMaxCapacity   = 20000
)

// ParamsMetric 携带频繁(热点)参数的实时计数器。
// 对于每个缓存映射，键是参数值，而值是计数器。
type ParamsMetric struct {
	RuleTimeCounter    cache.ConcurrentCounterCache // 记录最后添加的令牌时间戳。
	RuleTokenCounter   cache.ConcurrentCounterCache // 记录令牌的数量。
	ConcurrencyCounter cache.ConcurrentCounterCache // 实时并发
}

package base

const (
	TotalInBoundResourceName        = "__total_inbound_traffic__"
	DefaultMaxResourceAmount uint32 = 10000
	DefaultSampleCount       uint32 = 2            // 默认是两个桶
	DefaultIntervalMs        uint32 = 1000         // 默认是监控1000ms内的请求
	DefaultSampleCountTotal  uint32 = 20           // default 10*1000/500 = 20
	DefaultIntervalMsTotal   uint32 = 10000        // default 10s (total length)
	DefaultStatisticMaxRt           = int64(60000) // 最大的请求时间
)

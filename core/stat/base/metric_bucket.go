package base

import (
	"sync/atomic"

	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/logging"
	"github.com/pkg/errors"
)

// MetricBucket 表示每个最小时间单位(即桶时间跨度)记录度量的实体.
// 注意MetricBucket的所有操作都要求是线程安全的.   // 滑动窗口中的每个桶
type MetricBucket struct {
	counter        [base.MetricEventTotal]int64 // 指标统计值, 数组
	minRt          int64                        // 最小的请求时间
	maxConcurrency int32                        // 最大并发量
}

func NewMetricBucket() *MetricBucket {
	mb := &MetricBucket{
		minRt:          base.DefaultStatisticMaxRt, // 最大的请求时间
		maxConcurrency: 0,                          //
	}
	return mb
}

// Add 添加给定度量事件的统计计数
func (mb *MetricBucket) Add(event base.MetricEvent, count int64) {
	if event >= base.MetricEventTotal || event < 0 {
		logging.Error(errors.Errorf("Unknown metric event: %v", event), "")
		return
	}
	if event == base.MetricEventRt {
		mb.AddRt(count)
		return
	}
	mb.addCount(event, count)
}

// 重中之重 ✈️✈️✈️✈️✈️✈️✈️✈️✈️✈️✈️✈️✈️✈️✈️✈️✈️✈️✈️✈️✈️✈️✈️✈️✈️✈️✈️✈️✈️
func (mb *MetricBucket) addCount(event base.MetricEvent, count int64) {
	atomic.AddInt64(&mb.counter[event], count)
}

// Get 获取给定度量事件的当前统计计数.
func (mb *MetricBucket) Get(event base.MetricEvent) int64 {
	if event >= base.MetricEventTotal || event < 0 {
		logging.Error(errors.Errorf("Unknown metric event: %v", event), "")
		return 0
	}
	return atomic.LoadInt64(&mb.counter[event])
}

func (mb *MetricBucket) reset() {
	for i := 0; i < int(base.MetricEventTotal); i++ {
		atomic.StoreInt64(&mb.counter[i], 0)
	}
	atomic.StoreInt64(&mb.minRt, base.DefaultStatisticMaxRt)
	atomic.StoreInt32(&mb.maxConcurrency, int32(0))
}

func (mb *MetricBucket) AddRt(rt int64) {
	mb.addCount(base.MetricEventRt, rt)
	if rt < atomic.LoadInt64(&mb.minRt) {
		// Might not be accurate here.
		atomic.StoreInt64(&mb.minRt, rt)
	}
}

func (mb *MetricBucket) MinRt() int64 {
	return atomic.LoadInt64(&mb.minRt)
}

func (mb *MetricBucket) UpdateConcurrency(concurrency int32) {
	cc := concurrency
	if cc > atomic.LoadInt32(&mb.maxConcurrency) {
		atomic.StoreInt32(&mb.maxConcurrency, cc)
	}
}

func (mb *MetricBucket) MaxConcurrency() int32 {
	return atomic.LoadInt32(&mb.maxConcurrency)
}

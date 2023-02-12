package base

import (
	"reflect"
	"sync/atomic"

	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/logging"
	"github.com/alibaba/sentinel-golang/util"
	"github.com/pkg/errors"
)

// SlidingWindowMetric 表示滑动窗口度量的包装器.
// 它不存储任何数据，是BucketLeapArray的包装器，以适应不同的内部桶.
// SlidingWindowMetric被设计为Sentinel功能的高级只读统计结构
type SlidingWindowMetric struct {
	bucketLengthInMs uint32 // 每个bucket内的时间长度
	sampleCount      uint32 // 一个窗口中的采样次数，或者叫bucket数
	intervalInMs     uint32 // 统计的窗口大小，单位毫秒
	real             *BucketLeapArray
}

// NewSlidingWindowMetric 指向内部统计BucketLeapArray的指针应该是有效的.
func NewSlidingWindowMetric(sampleCount, intervalInMs uint32, real *BucketLeapArray) (*SlidingWindowMetric, error) {
	if real == nil {
		return nil, errors.New("nil BucketLeapArray")
	}
	if err := base.CheckValidityForReuseStatistic(sampleCount, intervalInMs, real.SampleCount(), real.IntervalInMs()); err != nil {
		return nil, err
	}
	bucketLengthInMs := intervalInMs / sampleCount // 每个bucket内的时间长度

	return &SlidingWindowMetric{
		bucketLengthInMs: bucketLengthInMs,
		sampleCount:      sampleCount,
		intervalInMs:     intervalInMs,
		real:             real,
	}, nil
}

// getBucketStartRange返回桶的起始时间范围.
// 实际的时间跨度是:[start, end + in.bucketTimeLength)
// 根据当前时间获取整个周期对应的窗口的开始时间和结束时间
func (m *SlidingWindowMetric) getBucketStartRange(timeMs uint64) (start, end uint64) {
	curBucketStartTime := calculateStartTime(timeMs, m.real.BucketLengthInMs())
	end = curBucketStartTime
	start = end - uint64(m.intervalInMs) + uint64(m.real.BucketLengthInMs())
	return
}

func (m *SlidingWindowMetric) getIntervalInSecond() float64 {
	return float64(m.intervalInMs) / 1000.0
}

func (m *SlidingWindowMetric) count(event base.MetricEvent, values []*BucketWrap) int64 {
	ret := int64(0)
	for _, ww := range values {
		mb := ww.Value.Load()
		if mb == nil {
			logging.Error(errors.New("nil BucketWrap"), "Current bucket value is nil in SlidingWindowMetric.count()")
			continue
		}
		counter, ok := mb.(*MetricBucket)
		if !ok {
			logging.Error(errors.New("type assert failed"), "Fail to do type assert in SlidingWindowMetric.count()", "expectType", "*MetricBucket", "actualType", reflect.TypeOf(mb).Name())
			continue
		}
		ret += counter.Get(event)
	}
	return ret
}

func (m *SlidingWindowMetric) GetSum(event base.MetricEvent) int64 {
	return m.getSumWithTime(util.CurrentTimeMillis(), event)
}

// 某种事件，当前缓存时间
func (m *SlidingWindowMetric) getSumWithTime(now uint64, event base.MetricEvent) int64 {
	satisfiedBuckets := m.getSatisfiedBuckets(now) // 根据时间,获取相对的buckets
	return m.count(event, satisfiedBuckets)        // 统计相应的数据
}

func (m *SlidingWindowMetric) GetQPS(event base.MetricEvent) float64 {
	return m.getQPSWithTime(util.CurrentTimeMillis(), event)
}

func (m *SlidingWindowMetric) GetPreviousQPS(event base.MetricEvent) float64 {
	return m.getQPSWithTime(util.CurrentTimeMillis()-uint64(m.bucketLengthInMs), event)
}

func (m *SlidingWindowMetric) getQPSWithTime(now uint64, event base.MetricEvent) float64 {
	return float64(m.getSumWithTime(now, event)) / m.getIntervalInSecond()
}

// 根据当前时间获取周期内的所有窗口
func (m *SlidingWindowMetric) getSatisfiedBuckets(now uint64) []*BucketWrap {
	start, end := m.getBucketStartRange(now)
	//提取startTime在[start, end]之间的桶
	//表示桶的时间视图为[firstStart, endStart+bucketLength]
	satisfiedBuckets := m.real.ValuesConditional(now, func(ws uint64) bool {
		return ws >= start && ws <= end
	})
	return satisfiedBuckets
}

func (m *SlidingWindowMetric) GetMaxOfSingleBucket(event base.MetricEvent) int64 {
	now := util.CurrentTimeMillis()
	satisfiedBuckets := m.getSatisfiedBuckets(now)
	var curMax int64 = 0
	for _, w := range satisfiedBuckets {
		mb := w.Value.Load()
		if mb == nil {
			logging.Error(errors.New("nil BucketWrap"), "Current bucket value is nil in SlidingWindowMetric.GetMaxOfSingleBucket()")
			continue
		}
		counter, ok := mb.(*MetricBucket)
		if !ok {
			logging.Error(errors.New("type assert failed"), "Fail to do type assert in SlidingWindowMetric.GetMaxOfSingleBucket()", "expectType", "*MetricBucket", "actualType", reflect.TypeOf(mb).Name())
			continue
		}
		v := counter.Get(event)
		if v > curMax {
			curMax = v
		}
	}
	return curMax
}

func (m *SlidingWindowMetric) MinRT() float64 {
	now := util.CurrentTimeMillis()
	satisfiedBuckets := m.getSatisfiedBuckets(now)
	minRt := base.DefaultStatisticMaxRt
	for _, w := range satisfiedBuckets {
		mb := w.Value.Load()
		if mb == nil {
			logging.Error(errors.New("nil BucketWrap"), "Current bucket value is nil in SlidingWindowMetric.MinRT()")
			continue
		}
		counter, ok := mb.(*MetricBucket)
		if !ok {
			logging.Error(errors.New("type assert failed"), "Fail to do type assert in SlidingWindowMetric.MinRT()", "expectType", "*MetricBucket", "actualType", reflect.TypeOf(mb).Name())
			continue
		}
		v := counter.MinRt()
		if v < minRt {
			minRt = v
		}
	}
	if minRt < 1 {
		minRt = 1
	}
	return float64(minRt)
}

func (m *SlidingWindowMetric) MaxConcurrency() int32 {
	now := util.CurrentTimeMillis()
	satisfiedBuckets := m.getSatisfiedBuckets(now)
	maxConcurrency := int32(0)
	for _, w := range satisfiedBuckets {
		mb := w.Value.Load()
		if mb == nil {
			logging.Error(errors.New("nil BucketWrap"), "Current bucket value is nil in SlidingWindowMetric.MaxConcurrency()")
			continue
		}
		counter, ok := mb.(*MetricBucket)
		if !ok {
			logging.Error(errors.New("type assert failed"), "Fail to do type assert in SlidingWindowMetric.MaxConcurrency()", "expectType", "*MetricBucket", "actualType", reflect.TypeOf(mb).Name())
			continue
		}
		v := counter.MaxConcurrency()
		if v > maxConcurrency {
			maxConcurrency = v
		}
	}
	return maxConcurrency
}

func (m *SlidingWindowMetric) AvgRT() float64 {
	return float64(m.GetSum(base.MetricEventRt)) / float64(m.GetSum(base.MetricEventComplete))
}

// SecondMetricsOnCondition 在统计桶的startTime满足时间谓词的条件下，以秒为单位聚合度量项。
func (m *SlidingWindowMetric) SecondMetricsOnCondition(predicate base.TimePredicate) []*base.MetricItem {
	ws := m.real.ValuesConditional(util.CurrentTimeMillis(), predicate)

	// Aggregate second-level MetricItem (only for stable metrics)
	wm := make(map[uint64][]*BucketWrap, 8)
	for _, w := range ws {
		bucketStart := atomic.LoadUint64(&w.BucketStart)
		secStart := bucketStart - bucketStart%1000
		if arr, hasData := wm[secStart]; hasData {
			wm[secStart] = append(arr, w)
		} else {
			wm[secStart] = []*BucketWrap{w}
		}
	}
	items := make([]*base.MetricItem, 0, 8)
	for ts, values := range wm {
		if len(values) == 0 {
			continue
		}
		if item := m.metricItemFromBuckets(ts, values); item != nil {
			items = append(items, item)
		}
	}
	return items
}

// metricItemFromBuckets 聚合多个桶包装器(基于相同的startTime，单位为秒)到单个MetricItem。
func (m *SlidingWindowMetric) metricItemFromBuckets(ts uint64, ws []*BucketWrap) *base.MetricItem {
	item := &base.MetricItem{Timestamp: ts}
	var allRt int64 = 0
	for _, w := range ws {
		mi := w.Value.Load()
		if mi == nil {
			logging.Error(errors.New("nil BucketWrap"), "Current bucket value is nil in SlidingWindowMetric.metricItemFromBuckets()")
			return nil
		}
		mb, ok := mi.(*MetricBucket)
		if !ok {
			logging.Error(errors.New("type assert failed"), "Fail to do type assert in SlidingWindowMetric.metricItemFromBuckets()", "bucketStartTime", w.BucketStart, "expectType", "*MetricBucket", "actualType", reflect.TypeOf(mb).Name())
			return nil
		}
		item.PassQps += uint64(mb.Get(base.MetricEventPass))
		item.BlockQps += uint64(mb.Get(base.MetricEventBlock))
		item.ErrorQps += uint64(mb.Get(base.MetricEventError))
		item.CompleteQps += uint64(mb.Get(base.MetricEventComplete))
		mc := uint32(mb.MaxConcurrency())
		if mc > item.Concurrency {
			item.Concurrency = mc
		}
		allRt += mb.Get(base.MetricEventRt)
	}
	if item.CompleteQps > 0 {
		item.AvgRt = uint64(allRt) / item.CompleteQps
	} else {
		item.AvgRt = uint64(allRt)
	}
	return item
}

func (m *SlidingWindowMetric) metricItemFromBucket(w *BucketWrap) *base.MetricItem {
	mi := w.Value.Load()
	if mi == nil {
		logging.Error(errors.New("nil BucketWrap"), "Current bucket value is nil in SlidingWindowMetric.metricItemFromBucket()")
		return nil
	}
	mb, ok := mi.(*MetricBucket)
	if !ok {
		logging.Error(errors.New("type assert failed"), "Fail to do type assert in SlidingWindowMetric.metricItemFromBucket()", "expectType", "*MetricBucket", "actualType", reflect.TypeOf(mb).Name())
		return nil
	}
	completeQps := mb.Get(base.MetricEventComplete)
	item := &base.MetricItem{
		PassQps:     uint64(mb.Get(base.MetricEventPass)),
		BlockQps:    uint64(mb.Get(base.MetricEventBlock)),
		ErrorQps:    uint64(mb.Get(base.MetricEventError)),
		CompleteQps: uint64(completeQps),
		Timestamp:   w.BucketStart,
	}
	if completeQps > 0 {
		item.AvgRt = uint64(mb.Get(base.MetricEventRt) / completeQps)
	} else {
		item.AvgRt = uint64(mb.Get(base.MetricEventRt))
	}
	return item
}

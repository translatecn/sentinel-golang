package base

import (
	"reflect"
	"sync/atomic"

	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/logging"
	"github.com/alibaba/sentinel-golang/util"
	"github.com/pkg/errors"
)

// BucketLeapArray 是基于LeapArray的滑动窗口实现(作为滑动窗口基础设施)
// 和MetricBucket(作为数据类型).MetricBucket用于记录统计信息
// 每个最小时间单位(即桶时间跨度)的度量.
type BucketLeapArray struct {
	data     LeapArray
	dataType string
}

func (bla *BucketLeapArray) NewEmptyBucket() interface{} {
	return NewMetricBucket()
}

func (bla *BucketLeapArray) ResetBucketTo(bw *BucketWrap, startTime uint64) *BucketWrap {
	atomic.StoreUint64(&bw.BucketStart, startTime)
	bw.Value.Store(NewMetricBucket())
	return bw
}

// NewBucketLeapArray 创建一个具有给定属性的BucketLeapArray.
// bucketCount 表示桶数，intervalInMs表示滑动窗口的总时间跨度.
func NewBucketLeapArray(bucketCount uint32, intervalInMs uint32) *BucketLeapArray {
	bucketLengthInMs := intervalInMs / bucketCount // 每个bucket的事件长度
	ret := &BucketLeapArray{
		data: LeapArray{
			bucketLengthInMs: bucketLengthInMs,
			bucketCount:      bucketCount,
			intervalInMs:     intervalInMs,
			array:            nil,
		},
		dataType: "MetricBucket",
	}
	arr := NewAtomicBucketWrapArray(int(bucketCount), bucketLengthInMs, ret)
	ret.data.array = arr
	return ret
}

func (bla *BucketLeapArray) SampleCount() uint32 {
	return bla.data.bucketCount
}

func (bla *BucketLeapArray) IntervalInMs() uint32 {
	return bla.data.intervalInMs
}

func (bla *BucketLeapArray) BucketLengthInMs() uint32 {
	return bla.data.bucketLengthInMs
}

func (bla *BucketLeapArray) DataType() string {
	return bla.dataType
}

func (bla *BucketLeapArray) GetIntervalInSecond() float64 {
	return float64(bla.IntervalInMs()) / 1000.0
}

func (bla *BucketLeapArray) AddCount(event base.MetricEvent, count int64) {
	bla.addCountWithTime(util.CurrentTimeMillis(), event, count)
}

func (bla *BucketLeapArray) addCountWithTime(now uint64, event base.MetricEvent, count int64) {
	b := bla.currentBucketWithTime(now)
	if b == nil {
		return
	}
	b.Add(event, count)
}

func (bla *BucketLeapArray) UpdateConcurrency(concurrency int32) {
	bla.updateConcurrencyWithTime(util.CurrentTimeMillis(), concurrency)
}

func (bla *BucketLeapArray) updateConcurrencyWithTime(now uint64, concurrency int32) {
	b := bla.currentBucketWithTime(now)
	if b == nil {
		return
	}
	b.UpdateConcurrency(concurrency)
}

func (bla *BucketLeapArray) currentBucketWithTime(now uint64) *MetricBucket {
	curBucket, err := bla.data.currentBucketOfTime(now, bla)
	if err != nil {
		logging.Error(err, "Failed to get current bucket in BucketLeapArray.currentBucketWithTime()", "now", now)
		return nil
	}
	if curBucket == nil {
		logging.Error(errors.New("current bucket is nil"), "Nil curBucket in BucketLeapArray.currentBucketWithTime()")
		return nil
	}
	mb := curBucket.Value.Load()
	if mb == nil {
		logging.Error(errors.New("nil bucket"), "Current bucket atomic Value is nil in BucketLeapArray.currentBucketWithTime()")
		return nil
	}
	b, ok := mb.(*MetricBucket)
	if !ok {
		logging.Error(errors.New("fail to type assert"), "Bucket data type error in BucketLeapArray.currentBucketWithTime()", "expectType", "*MetricBucket", "actualType", reflect.TypeOf(mb).Name())
		return nil
	}
	return b
}

// Count returns the sum count for the given MetricEvent within all valid (non-expired) buckets.
func (bla *BucketLeapArray) Count(event base.MetricEvent) int64 {
	// it might panic?
	return bla.CountWithTime(util.CurrentTimeMillis(), event)
}

func (bla *BucketLeapArray) CountWithTime(now uint64, event base.MetricEvent) int64 {
	_, err := bla.data.currentBucketOfTime(now, bla)
	if err != nil {
		logging.Error(err, "Failed to get current bucket in BucketLeapArray.CountWithTime()", "now", now)
	}
	count := int64(0)
	for _, ww := range bla.data.valuesWithTime(now) {
		mb := ww.Value.Load()
		if mb == nil {
			logging.Error(errors.New("current bucket is nil"), "Failed to load current bucket in BucketLeapArray.CountWithTime()")
			continue
		}
		b, ok := mb.(*MetricBucket)
		if !ok {
			logging.Error(errors.New("fail to type assert"), "Bucket data type error in BucketLeapArray.CountWithTime()", "expectType", "*MetricBucket", "actualType", reflect.TypeOf(mb).Name())
			continue
		}
		count += b.Get(event)
	}
	return count
}

// Values returns all valid (non-expired) buckets.
func (bla *BucketLeapArray) Values(now uint64) []*BucketWrap {
	// Refresh current bucket if necessary.
	_, err := bla.data.currentBucketOfTime(now, bla)
	if err != nil {
		logging.Error(err, "Failed to refresh current bucket in BucketLeapArray.Values()", "now", now)
	}

	return bla.data.valuesWithTime(now)
}

// ValuesConditional 匹配符合条件的窗口
func (bla *BucketLeapArray) ValuesConditional(now uint64, predicate base.TimePredicate) []*BucketWrap {
	return bla.data.ValuesConditional(now, predicate)
}

func (bla *BucketLeapArray) MinRt() int64 {
	_, err := bla.data.CurrentBucket(bla)
	if err != nil {
		logging.Error(err, "Failed to get current bucket in BucketLeapArray.MinRt()")
	}

	ret := base.DefaultStatisticMaxRt

	for _, v := range bla.data.Values() {
		mb := v.Value.Load()
		if mb == nil {
			logging.Error(errors.New("current bucket is nil"), "Failed to load current bucket in BucketLeapArray.MinRt()")
			continue
		}
		b, ok := mb.(*MetricBucket)
		if !ok {
			logging.Error(errors.New("fail to type assert"), "Bucket data type error in BucketLeapArray.MinRt()", "expectType", "*MetricBucket", "actualType", reflect.TypeOf(mb).Name())
			continue
		}
		mr := b.MinRt()
		if ret > mr {
			ret = mr
		}
	}
	return ret
}

func (bla *BucketLeapArray) MaxConcurrency() int32 {
	_, err := bla.data.CurrentBucket(bla)
	if err != nil {
		logging.Error(err, "Failed to get current bucket in BucketLeapArray.MaxConcurrency()")
	}

	ret := int32(0)

	for _, v := range bla.data.Values() {
		mb := v.Value.Load()
		if mb == nil {
			logging.Error(errors.New("current bucket is nil"), "Failed to load current bucket in BucketLeapArray.MaxConcurrency()")
			continue
		}
		b, ok := mb.(*MetricBucket)
		if !ok {
			logging.Error(errors.New("fail to type assert"), "Bucket data type error in BucketLeapArray.MaxConcurrency()", "expectType", "*MetricBucket", "actualType", reflect.TypeOf(mb).Name())
			continue
		}
		mc := b.MaxConcurrency()
		if ret < mc {
			ret = mc
		}
	}
	return ret
}

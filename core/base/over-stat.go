package base

import (
	"errors"
)

type TimePredicate func(uint64) bool

type MetricEvent int8

const (
	MetricEventPass     MetricEvent = iota // 哨点规则检查通过
	MetricEventBlock                       //
	MetricEventComplete                    //
	MetricEventError                       // Biz误差，用于断路器
	MetricEventRt                          // 请求执行rt，单位为毫秒
	MetricEventTotal                       // hack事件的数量
)

var (
	globalNopReadStat  = &nopReadStat{}
	globalNopWriteStat = &nopWriteStat{}
)

type ReadStat interface {
	GetQPS(event MetricEvent) float64         // QPS请求量
	GetPreviousQPS(event MetricEvent) float64 // 获取之前的请求QPS
	GetSum(event MetricEvent) int64           // 获取当前统计周期内已通过的请求数量
	MinRT() float64                           // 最小的请求时间
	AvgRT() float64                           // 平均请求时间
}

func NopReadStat() *nopReadStat {
	return globalNopReadStat
}

type nopReadStat struct {
}

func (rs *nopReadStat) GetQPS(_ MetricEvent) float64 {
	return 0.0
}

func (rs *nopReadStat) GetPreviousQPS(_ MetricEvent) float64 {
	return 0.0
}

func (rs *nopReadStat) GetSum(_ MetricEvent) int64 {
	return 0
}

func (rs *nopReadStat) MinRT() float64 {
	return 0.0
}

func (rs *nopReadStat) AvgRT() float64 {
	return 0.0
}

type WriteStat interface {
	AddCount(event MetricEvent, count int64) // 将给定的计数添加到提供的MetricEvent的度量中.
}

func NopWriteStat() *nopWriteStat {
	return globalNopWriteStat
}

type nopWriteStat struct {
}

func (ws *nopWriteStat) AddCount(_ MetricEvent, _ int64) {
}

// ConcurrencyStat 提供并发统计的读/更新操作.
type ConcurrencyStat interface {
	CurrentConcurrency() int32
	IncreaseConcurrency()
	DecreaseConcurrency()
}

// StatNode holds real-time statistics for resources.
type StatNode interface {
	MetricItemRetriever
	ReadStat
	WriteStat
	ConcurrencyStat
	GenerateReadStat(sampleCount uint32, intervalInMs uint32) (ReadStat, error)
}

var (
	IllegalGlobalStatisticParamsError = errors.New("Invalid parameters, sampleCount or interval, for resource's global statistic")
	IllegalStatisticParamsError       = errors.New("Invalid parameters, sampleCount or interval, for metric statistic")
	GlobalStatisticNonReusableError   = errors.New("The parameters, sampleCount and interval, mismatch for reusing between resource's global statistic and readonly metric statistic.")
)

func CheckValidityForStatistic(sampleCount, intervalInMs uint32) error {
	if intervalInMs == 0 || sampleCount == 0 || intervalInMs%sampleCount != 0 {
		return IllegalStatisticParamsError
	}
	return nil
}

// CheckValidityForReuseStatistic 👌🏻
func CheckValidityForReuseStatistic(sampleCount, intervalInMs uint32, parentSampleCount, parentIntervalInMs uint32) error {
	if intervalInMs == 0 || sampleCount == 0 || intervalInMs%sampleCount != 0 {
		return IllegalStatisticParamsError
	}
	bucketLengthInMs := intervalInMs / sampleCount // 每个bucket的长度

	if parentIntervalInMs == 0 || parentSampleCount == 0 || parentIntervalInMs%parentSampleCount != 0 {
		return IllegalGlobalStatisticParamsError
	}
	parentBucketLengthInMs := parentIntervalInMs / parentSampleCount

	if parentIntervalInMs%intervalInMs != 0 {
		return GlobalStatisticNonReusableError
	}
	if bucketLengthInMs%parentBucketLengthInMs != 0 {
		return GlobalStatisticNonReusableError
	}
	return nil
}

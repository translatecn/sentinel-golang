package base

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"unsafe"

	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/logging"
	"github.com/alibaba/sentinel-golang/util"
	"github.com/pkg/errors"
)

// BucketWrap 表示记录度量的槽。
// 为了减少内存占用，BucketWrap不保存桶的长度。
// 在LeapArray中可以看到BucketWrap的长度。
// 时间范围为[startTime, startTime+bucketLength]。
// BucketWrap的大小为24(8+16)字节。
type BucketWrap struct {
	BucketStart uint64
	Value       atomic.Value
}

func (ww *BucketWrap) resetTo(startTime uint64) {
	ww.BucketStart = startTime
}

func (ww *BucketWrap) isTimeInBucket(now uint64, bucketLengthInMs uint32) bool {
	return ww.BucketStart <= now && now < ww.BucketStart+uint64(bucketLengthInMs)
}

// 计算开始时间
func calculateStartTime(now uint64, bucketLengthInMs uint32) uint64 {
	return now - (now % uint64(bucketLengthInMs))
}

// AtomicBucketWrapArray 表示线程安全的循环数组.
// 数组的长度应该在创建时提供，不能被修改.
type AtomicBucketWrapArray struct {
	base   unsafe.Pointer // .data内部结构SliceHeader 中Data的地址
	length int            // 窗口数组的长度
	data   []*BucketWrap  //窗口数组
}

func NewAtomicBucketWrapArrayWithTime(len int, bucketLengthInMs uint32, now uint64, generator BucketGenerator) *AtomicBucketWrapArray {
	ret := &AtomicBucketWrapArray{
		length: len,
		data:   make([]*BucketWrap, len),
	}
	// 窗口下标位置
	// bucketLengthInMs 每个bucket的时间长度
	idx := int((now / uint64(bucketLengthInMs)) % uint64(len))
	startTime := calculateStartTime(now, bucketLengthInMs)

	for i := idx; i <= len-1; i++ {
		ww := &BucketWrap{
			BucketStart: startTime,
			Value:       atomic.Value{},
		}
		ww.Value.Store(generator.NewEmptyBucket())
		ret.data[i] = ww
		startTime += uint64(bucketLengthInMs)
	}
	for i := 0; i < idx; i++ {
		ww := &BucketWrap{
			BucketStart: startTime,
			Value:       atomic.Value{},
		}
		ww.Value.Store(generator.NewEmptyBucket())
		ret.data[i] = ww
		startTime += uint64(bucketLengthInMs)
	}

	// calculate base address for real data array
	//3:将窗口数组首元素地址设置到原子时间轮
	//这么做的原因主要是实现对时间轮中的元素（窗口）进行原子无锁的读取和更新，极大的提升性能.
	sliHeader := (*util.SliceHeader)(unsafe.Pointer(&ret.data))
	ret.base = unsafe.Pointer((**BucketWrap)(unsafe.Pointer(sliHeader.Data)))
	return ret
}

// NewAtomicBucketWrapArray 创建一个AtomicBucketWrapArray，初始化每个BucketWrap的数据.
// len表示桶的长度.bucketLengthInMs表示每个桶的桶长(单位为毫秒).
// 生成器接受BucketGenerator来生成和刷新桶.
func NewAtomicBucketWrapArray(len int, bucketLengthInMs uint32, generator BucketGenerator) *AtomicBucketWrapArray {
	return NewAtomicBucketWrapArrayWithTime(len, bucketLengthInMs, util.CurrentTimeMillis(), generator)
}

// 获取对应窗口的地址
func (aa *AtomicBucketWrapArray) elementOffset(idx int) (unsafe.Pointer, bool) {
	if idx >= aa.length || idx < 0 {
		logging.Error(errors.New("array index out of bounds"),
			"array index out of bounds in AtomicBucketWrapArray.elementOffset()",
			"idx", idx, "arrayLength", aa.length)
		return nil, false
	}
	basePtr := aa.base
	return unsafe.Pointer(uintptr(basePtr) + uintptr(idx)*unsafe.Sizeof(basePtr)), true
}

// 获取对应窗口
func (aa *AtomicBucketWrapArray) get(idx int) *BucketWrap {
	if offset, ok := aa.elementOffset(idx); ok {
		return (*BucketWrap)(atomic.LoadPointer((*unsafe.Pointer)(offset)))
	}
	return nil
}

// 替换对应窗口
func (aa *AtomicBucketWrapArray) compareAndSet(idx int, except, update *BucketWrap) bool {
	// aa.elementOffset(idx) return the secondary pointer of BucketWrap, which is the pointer to the aa.data[idx]
	// then convert to (*unsafe.Pointer)
	// update secondary pointer
	if offset, ok := aa.elementOffset(idx); ok {
		return atomic.CompareAndSwapPointer((*unsafe.Pointer)(offset), unsafe.Pointer(except), unsafe.Pointer(update))
	}
	return false
}

// LeapArray 表示滑动窗口数据结构的基本实现
//
// For example, assuming bucketCount=5, intervalInMs is 1000ms, so the bucketLength is 200ms.
// Let's give a diagram to illustrate.
// Suppose current timestamp is 1188, bucketLength is 200ms, intervalInMs is 1000ms, then
// time span of current bucket is [1000, 1200). The representation of the underlying structure:
//
//	 B0       B1      B2     B3      B4
//	 |_______|_______|_______|_______|_______|
//	1000    1200    400     600     800    (1000) ms
//	       ^
//	    time=1188
type LeapArray struct {
	bucketLengthInMs uint32                 //
	bucketCount      uint32                 // 表示BucketWrap的个数.
	intervalInMs     uint32                 // 表示滑动窗口的总时间跨度(以毫秒为单位).
	array            *AtomicBucketWrapArray // 表示内部圆形数组.
	updateLock       mutex                  // 更新操作的内部锁.
}

func NewLeapArray(bucketCount uint32, intervalInMs uint32, generator BucketGenerator) (*LeapArray, error) {
	if bucketCount == 0 || intervalInMs%bucketCount != 0 {
		return nil, errors.Errorf("Invalid parameters, intervalInMs is %d, bucketCount is %d", intervalInMs, bucketCount)
	}
	if generator == nil {
		return nil, errors.Errorf("Invalid parameters, BucketGenerator is nil")
	}
	bucketLengthInMs := intervalInMs / bucketCount // 每个bucket的长度
	return &LeapArray{
		bucketLengthInMs: bucketLengthInMs,
		bucketCount:      bucketCount,
		intervalInMs:     intervalInMs,
		array:            NewAtomicBucketWrapArray(int(bucketCount), bucketLengthInMs, generator),
	}, nil
}

func (la *LeapArray) CurrentBucket(bg BucketGenerator) (*BucketWrap, error) {
	return la.currentBucketOfTime(util.CurrentTimeMillis(), bg)
}

func (la *LeapArray) currentBucketOfTime(now uint64, bg BucketGenerator) (*BucketWrap, error) {
	if now <= 0 {
		return nil, errors.New("Current time is less than 0.")
	}

	// 计算当前时间对应的窗口下标
	idx := la.calculateTimeIdx(now)
	// 计算当前时间对应的窗口的开始时间
	bucketStart := calculateStartTime(now, la.bucketLengthInMs)

	for { //spin to get the current BucketWrap
		// 获取旧窗口
		old := la.array.get(idx)
		// 如果旧窗口==nil则初始化(正常不会执行这部分代码)
		if old == nil {
			// because la.array.data had initiated when new la.array
			// theoretically, here is not reachable
			newWrap := &BucketWrap{
				BucketStart: bucketStart,
				Value:       atomic.Value{},
			}
			newWrap.Value.Store(bg.NewEmptyBucket())
			if la.array.compareAndSet(idx, nil, newWrap) {
				return newWrap, nil
			} else {
				runtime.Gosched()
			}

		} else if bucketStart == atomic.LoadUint64(&old.BucketStart) { // 如果本次计算的开始时间等于旧窗口的开始时间，则认为窗口没有过期，直接返回
			return old, nil

		} else if bucketStart > atomic.LoadUint64(&old.BucketStart) { //  如果本次计算的开始时间大于旧窗口的开始时间，则认为窗口过期尝试重置
			// current time has been next cycle of LeapArray and LeapArray dont't count in last cycle.
			// reset BucketWrap
			if la.updateLock.TryLock() {
				old = bg.ResetBucketTo(old, bucketStart)
				la.updateLock.Unlock()
				return old, nil
			} else {
				runtime.Gosched()
			}
		} else if bucketStart < atomic.LoadUint64(&old.BucketStart) {
			if la.bucketCount == 1 {
				// if bucketCount==1 in leap array, in concurrency scenario, this case is possible
				return old, nil
			}
			// TODO: reserve for some special case (e.g. when occupying "future" buckets).
			return nil, errors.New(fmt.Sprintf("Provided time timeMillis=%d is already behind old.BucketStart=%d.", bucketStart, old.BucketStart))
		}
	}
}

func (la *LeapArray) calculateTimeIdx(now uint64) int {
	timeId := now / uint64(la.bucketLengthInMs)
	return int(timeId) % la.array.length
}

// Values returns all valid (non-expired) buckets between [curBucketEnd-windowInterval, curBucketEnd],
// where curBucketEnd=curBucketStart+bucketLength.
func (la *LeapArray) Values() []*BucketWrap {
	return la.valuesWithTime(util.CurrentTimeMillis())
}

func (la *LeapArray) valuesWithTime(now uint64) []*BucketWrap {
	if now <= 0 {
		return make([]*BucketWrap, 0)
	}
	ret := make([]*BucketWrap, 0, la.array.length)
	for i := 0; i < la.array.length; i++ {
		ww := la.array.get(i)
		if ww == nil || la.isBucketDeprecated(now, ww) {
			continue
		}
		ret = append(ret, ww)
	}
	return ret
}

// ValuesConditional 返回startTimestamp满足给定时间戳条件(谓词)的所有桶。
// 函数使用参数"now"作为目标时间戳。
func (la *LeapArray) ValuesConditional(now uint64, predicate base.TimePredicate) []*BucketWrap {
	if now <= 0 {
		return make([]*BucketWrap, 0)
	}
	ret := make([]*BucketWrap, 0, la.array.length)
	for i := 0; i < la.array.length; i++ {
		ww := la.array.get(i)
		if ww == nil || la.isBucketDeprecated(now, ww) || !predicate(atomic.LoadUint64(&ww.BucketStart)) {
			continue
		}
		ret = append(ret, ww)
	}
	return ret
}

func (la *LeapArray) isBucketDeprecated(now uint64, ww *BucketWrap) bool {
	ws := atomic.LoadUint64(&ww.BucketStart)
	return (now - ws) > uint64(la.intervalInMs)
}

// BucketGenerator 表示用于生成和刷新桶的“通用”接口.
type BucketGenerator interface {
	NewEmptyBucket() interface{}
	ResetBucketTo(bucket *BucketWrap, startTime uint64) *BucketWrap
}

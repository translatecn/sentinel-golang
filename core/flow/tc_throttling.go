package flow

import (
	"math"
	"sync/atomic"
	"time"

	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/util"
)

const (
	BlockMsgQueueing = "flow throttling check blocked, estimated queueing time exceeds max queueing time"

	MillisToNanosOffset = int64(time.Millisecond / time.Nanosecond)
)

// ThrottlingChecker limits the time interval between two requests.
type ThrottlingChecker struct {
	owner             *TrafficShapingController
	maxQueueingTimeNs int64
	statIntervalNs    int64
	lastPassedTime    int64
}

func NewThrottlingChecker(owner *TrafficShapingController, timeoutMs uint32, statIntervalMs uint32) *ThrottlingChecker {
	var statIntervalNs int64
	if statIntervalMs == 0 {
		statIntervalNs = 1000 * MillisToNanosOffset
	} else {
		statIntervalNs = int64(statIntervalMs) * MillisToNanosOffset
	}
	return &ThrottlingChecker{
		owner:             owner,
		maxQueueingTimeNs: int64(timeoutMs) * MillisToNanosOffset,
		statIntervalNs:    statIntervalNs,
		lastPassedTime:    0,
	}
}
func (c *ThrottlingChecker) BoundOwner() *TrafficShapingController {
	return c.owner
}

// DoCheck 匀速排队
func (c *ThrottlingChecker) DoCheck(_ base.StatNode, batchCount uint32, threshold float64) *base.TokenResult {
	// Pass when batch count is less or equal than 0.
	if batchCount <= 0 {
		return nil
	}

	var rule *Rule
	if c.BoundOwner() != nil {
		rule = c.BoundOwner().BoundRule()
	}

	if threshold <= 0.0 {
		msg := "flow throttling check blocked, threshold is <= 0.0"
		return base.NewTokenResultBlockedWithCause(base.BlockTypeFlow, msg, rule, nil)
	}
	if float64(batchCount) > threshold {
		return base.NewTokenResultBlocked(base.BlockTypeFlow)
	}
	// Here we use nanosecond so that we could control the queueing time more accurately.
	// 获取当前时间(单位纳秒，方便更精确的计算排队时间)
	curNano := int64(util.CurrentTimeNano())

	// 计算每个流量排队的时间间隔，这里假设当前流量(batchCount=1),流控阈值(threshold=100),统计周期:单位纳秒(statIntervalNs=1000000000)
	intervalNs := int64(math.Ceil(float64(batchCount) / threshold * float64(c.statIntervalNs)))
	// 最后一个流量排队通过的时间
	loadedLastPassedTime := atomic.LoadInt64(&c.lastPassedTime)
	// 当前请求预计的通过时间=最后一个流量排队通过的时间+排队的时间间隔
	expectedTime := loadedLastPassedTime + intervalNs
	// 如果预计通过的时间小于当前时间，则说明不需要排队，直接放行。
	if expectedTime <= curNano {
		if swapped := atomic.CompareAndSwapInt64(&c.lastPassedTime, loadedLastPassedTime, curNano); swapped {
			// nil means pass
			return nil
		}
	}
	// 预估排队等待的时长=当前请求预计的通过时间-当前时间
	estimatedQueueingDuration := atomic.LoadInt64(&c.lastPassedTime) + intervalNs - curNano
	// 如果预估排队时长大于设定的最大等待时长，则直接被拒绝掉当前流量
	if estimatedQueueingDuration > c.maxQueueingTimeNs {
		return base.NewTokenResultBlockedWithCause(base.BlockTypeFlow, BlockMsgQueueing, rule, nil)
	}
	// 原子操作:得到当前流量的通过时间(这里主要避免并发导致lastPassedTime不是最新的)
	oldTime := atomic.AddInt64(&c.lastPassedTime, intervalNs)
	// 预估排队等待的时长
	estimatedQueueingDuration = oldTime - curNano
	if estimatedQueueingDuration > c.maxQueueingTimeNs {
		// 如果大于了设定的最大等待时长，这里减去排队间隔，因为不需要排队了，直接拒绝当前流量。
		atomic.AddInt64(&c.lastPassedTime, -intervalNs)
		return base.NewTokenResultBlockedWithCause(base.BlockTypeFlow, BlockMsgQueueing, rule, nil)
	}
	// 如果预估排队等待的时长大于0，则按照排队等待的时长进行sleep
	if estimatedQueueingDuration > 0 {
		return base.NewTokenResultShouldWait(time.Duration(estimatedQueueingDuration))
	} else {
		return base.NewTokenResultShouldWait(0)
	}
}

package base

import (
	"sort"
	"sync"

	"github.com/alibaba/sentinel-golang/logging"
	"github.com/alibaba/sentinel-golang/util"
	"github.com/pkg/errors"
)

type Slot interface {
	Order() uint32
}

// StatPrepareSlot 负责统计前的一些准备工作.例如:init结构等等
// 准备函数初始化
// 例如:init统计结构，节点等
// 准备的结果存储在EntryContext中
// 所有statprepareslot依次执行
// 准备函数不抛出panic.
type StatPrepareSlot interface {
	Slot
	Prepare(ctx *EntryContext)
}

// RuleCheckSlot 是基于规则的检查策略
// 所有检查规则都必须实现此接口.
type RuleCheckSlot interface {
	Slot
	// Check 检查函数进行一些验证,可以切断槽
	// 每个TokenResult将返回检查结果,上层逻辑将根据SlotResult控制管道.
	Check(ctx *EntryContext) *TokenResult
}

// StatSlot 负责统计所有自定义业务指标。
// StatSlot 将不处理任何panic，并将所有panic传递给slot chain
type StatSlot interface {
	Slot
	OnEntryPassed(ctx *EntryContext)                          // StatSlots将执行一些统计逻辑，例如QPS, log等
	OnEntryBlocked(ctx *EntryContext, blockError *BlockError) // StatSlots将执行一些统计逻辑，例如QPS, log等
	OnCompleted(ctx *EntryContext)                            // 将在chain退出时被调用。OnCompleted的语义是传递和完成的条目 注意:被阻塞的入口不会调用这个函数
}

// SlotChain 保持所有系统槽位和定制槽位.
// SlotChain 支持开发人员开发的插件槽.
type SlotChain struct {
	statPres   []StatPrepareSlot // 按StatPrepareSlot.Order()值升序排列.
	ruleChecks []RuleCheckSlot   // 按StatPrepareSlot.Order()值升序排列.,每个请求到来 都会经过至少5轮检查
	stats      []StatSlot        // 按StatPrepareSlot.Order()值升序排列.
	ctxPool    *sync.Pool        // EntryContext
}

var (
	ctxPool = &sync.Pool{
		New: func() interface{} {
			ctx := NewEmptyEntryContext()
			ctx.RuleCheckResult = NewTokenResultPass()
			ctx.Data = make(map[interface{}]interface{})
			ctx.Input = &SentinelInput{
				BatchCount:  1,
				Flag:        0,
				Args:        make([]interface{}, 0),
				Attachments: make(map[interface{}]interface{}),
			}
			return ctx
		},
	}
)

func NewSlotChain() *SlotChain {
	return &SlotChain{
		statPres:   make([]StatPrepareSlot, 0, 8),
		ruleChecks: make([]RuleCheckSlot, 0, 8),
		stats:      make([]StatSlot, 0, 8),
		ctxPool:    ctxPool,
	}
}

func (sc *SlotChain) GetPooledContext() *EntryContext {
	ctx := sc.ctxPool.Get().(*EntryContext)
	ctx.startTime = util.CurrentTimeMillis()
	return ctx
}

func (sc *SlotChain) RefurbishContext(c *EntryContext) {
	if c != nil {
		c.Reset()
		sc.ctxPool.Put(c)
	}
}

// AddStatPrepareSlot 将StatPrepareSlot槽位添加到SlotChain的StatPrepareSlot列表中.
// 列表中的所有StatPrepareSlot将按照StatPrepareSlot. order()从小到大排序.
// AddStatPrepareSlot是非线程安全的
// 并发场景下，AddStatPrepareSlot必须由SlotChain保护RWMutex
func (sc *SlotChain) AddStatPrepareSlot(s StatPrepareSlot) {
	sc.statPres = append(sc.statPres, s)
	sort.SliceStable(sc.statPres, func(i, j int) bool {
		return sc.statPres[i].Order() < sc.statPres[j].Order()
	})
}

func (sc *SlotChain) AddRuleCheckSlot(s RuleCheckSlot) {
	sc.ruleChecks = append(sc.ruleChecks, s)
	sort.SliceStable(sc.ruleChecks, func(i, j int) bool {
		return sc.ruleChecks[i].Order() < sc.ruleChecks[j].Order()
	})
}

func (sc *SlotChain) AddStatSlot(s StatSlot) {
	sc.stats = append(sc.stats, s)
	sort.SliceStable(sc.stats, func(i, j int) bool {
		return sc.stats[i].Order() < sc.stats[j].Order()
	})
}

// Entry 如果内部panic，返回TokenResult, nil.
func (sc *SlotChain) Entry(ctx *EntryContext) *TokenResult {
	// 这种情况不应该发生，除非哨兵内部存在错误.
	// 如果发生了，需要在EntryContext中添加TokenResult
	defer func() {
		if err := recover(); err != nil {
			logging.Error(errors.Errorf("%+v", err), "Sentinel internal panic in SlotChain.Entry()")
			ctx.SetError(errors.Errorf("%+v", err))
			return
		}
	}()

	// 执行准备槽
	sps := sc.statPres
	if len(sps) > 0 {
		for _, s := range sps {
			s.Prepare(ctx)
		}
	}

	// 执行基于规则的槽位检查
	rcs := sc.ruleChecks
	var ruleCheckRet *TokenResult
	if len(rcs) > 0 {
		for _, s := range rcs {
			sr := s.Check(ctx)
			if sr == nil {
				continue
			}
			if sr.IsBlocked() {
				ruleCheckRet = sr
				break
			}
		}
	}
	if ruleCheckRet == nil {
		ctx.RuleCheckResult.ResetToPass()
	} else {
		ctx.RuleCheckResult = ruleCheckRet
	}

	// 执行统计槽
	ss := sc.stats
	ruleCheckRet = ctx.RuleCheckResult
	if len(ss) > 0 {
		for _, s := range ss {
			if !ruleCheckRet.IsBlocked() { // 表示基于规则检查槽位的结果.
				s.OnEntryPassed(ctx)
			} else {
				s.OnEntryBlocked(ctx, ruleCheckRet.blockErr) // block 错误不应该是nil.
			}
		}
	}
	return ruleCheckRet
}

func (sc *SlotChain) exit(ctx *EntryContext) {
	if ctx == nil || ctx.Entry() == nil {
		logging.Error(errors.New("entryContext or SentinelEntry is nil"),
			"EntryContext or SentinelEntry is nil in SlotChain.exit()", "ctx", ctx)
		return
	}
	if ctx.IsBlocked() {
		return
	}
	for _, s := range sc.stats {
		s.OnCompleted(ctx)
	}
}

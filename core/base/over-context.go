package base

import "github.com/alibaba/sentinel-golang/util"

type EntryContext struct {
	entry           *SentinelEntry
	err             error
	startTime       uint64 // 用于计算RT
	rt              uint64 // 这笔交易的费用
	Resource        *ResourceWrapper
	StatNode        StatNode
	Input           *SentinelInput
	RuleCheckResult *TokenResult // 规则槽检查的结果
	Data            map[interface{}]interface{}
}

func (ctx *EntryContext) SetEntry(entry *SentinelEntry) {
	ctx.entry = entry
}

func (ctx *EntryContext) Entry() *SentinelEntry {
	return ctx.entry
}

func (ctx *EntryContext) Err() error {
	return ctx.err
}

func (ctx *EntryContext) SetError(err error) {
	ctx.err = err
}

func (ctx *EntryContext) StartTime() uint64 {
	return ctx.startTime
}

func (ctx *EntryContext) IsBlocked() bool {
	if ctx.RuleCheckResult == nil {
		return false
	}
	return ctx.RuleCheckResult.IsBlocked()
}

func (ctx *EntryContext) PutRt(rt uint64) {
	ctx.rt = rt
}

func (ctx *EntryContext) Rt() uint64 {
	if ctx.rt == 0 {
		rt := util.CurrentTimeMillis() - ctx.StartTime()
		return rt
	}
	return ctx.rt
}

func NewEmptyEntryContext() *EntryContext {
	return &EntryContext{}
}

// SentinelInput 哨兵的输入数据
type SentinelInput struct {
	BatchCount  uint32
	Flag        int32
	Args        []interface{}
	Attachments map[interface{}]interface{} // 当调用context in slot时，在此上下文中存储一些值.
}

func (i *SentinelInput) reset() {
	i.BatchCount = 1
	i.Flag = 0
	i.Args = i.Args[:0]
	if len(i.Attachments) != 0 {
		i.Attachments = make(map[interface{}]interface{})
	}
}

func (ctx *EntryContext) Reset() {
	ctx.entry = nil
	ctx.err = nil
	ctx.startTime = 0
	ctx.rt = 0
	ctx.Resource = nil
	ctx.StatNode = nil
	ctx.Input.reset()
	if ctx.RuleCheckResult == nil {
		ctx.RuleCheckResult = NewTokenResultPass()
	} else {
		ctx.RuleCheckResult.ResetToPass()
	}
	if len(ctx.Data) != 0 {
		ctx.Data = make(map[interface{}]interface{})
	}
}

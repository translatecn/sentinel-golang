package api

import (
	"sync"

	"github.com/alibaba/sentinel-golang/core/base"
)

var entryOptsPool = sync.Pool{
	New: func() interface{} {
		return &EntryOptions{
			resourceType: base.ResTypeCommon, //
			entryType:    base.Outbound,      // 流量类型
			batchCount:   1,
			flag:         0,
			slotChain:    nil,
			args:         nil,
			attachments:  nil,
		}
	},
}

// EntryOptions 表示哨兵资源条目的选项。
type EntryOptions struct {
	resourceType base.ResourceType
	entryType    base.TrafficType // 流量类型
	batchCount   uint32
	flag         int32
	slotChain    *base.SlotChain
	args         []interface{}
	attachments  map[interface{}]interface{}
}

func (o *EntryOptions) Reset() {
	o.resourceType = base.ResTypeCommon
	o.entryType = base.Outbound
	o.batchCount = 1
	o.flag = 0
	o.slotChain = nil
	o.args = o.args[:0]
	o.attachments = nil
}

type EntryOption func(*EntryOptions)

// WithResourceType 使用给定的资源类型设置资源项。
func WithResourceType(resourceType base.ResourceType) EntryOption {
	return func(opts *EntryOptions) {
		opts.resourceType = resourceType
	}
}

// WithTrafficType 使用给定的流量类型设置资源项。
func WithTrafficType(entryType base.TrafficType) EntryOption {
	return func(opts *EntryOptions) {
		opts.entryType = entryType
	}
}

// WithBatchCount 使用给定的批处理计数(默认为1)设置资源条目。
func WithBatchCount(batchCount uint32) EntryOption {
	return func(opts *EntryOptions) {
		opts.batchCount = batchCount
	}
}

func WithArgs(args ...interface{}) EntryOption {
	return func(opts *EntryOptions) {
		opts.args = append(opts.args, args...)
	}
}

func WithSlotChain(chain *base.SlotChain) EntryOption {
	return func(opts *EntryOptions) {
		opts.slotChain = chain
	}
}

func WithAttachments(data map[interface{}]interface{}) EntryOption {
	return func(opts *EntryOptions) {
		if opts.attachments == nil {
			opts.attachments = make(map[interface{}]interface{}, len(data))
		}
		for key, value := range data {
			opts.attachments[key] = value
		}
	}
}

// Entry 入站流量的入口
func Entry(resource string, opts ...EntryOption) (*base.SentinelEntry, *base.BlockError) {
	options := entryOptsPool.Get().(*EntryOptions)
	defer func() {
		options.Reset()
		entryOptsPool.Put(options)
	}()

	for _, opt := range opts {
		opt(options)
	}
	if options.slotChain == nil {
		options.slotChain = GlobalSlotChain()
	}
	return entry(resource, options)
}

// 记录指标数
func entry(resource string, options *EntryOptions) (*base.SentinelEntry, *base.BlockError) {
	rw := base.NewResourceWrapper(resource, options.resourceType, options.entryType)
	sc := options.slotChain

	if sc == nil {
		return base.NewSentinelEntry(nil, rw, nil), nil
	}
	// 从池中获取上下文。
	ctx := sc.GetPooledContext()
	ctx.Resource = rw
	ctx.Input.BatchCount = options.batchCount
	ctx.Input.Flag = options.flag
	if len(options.args) != 0 {
		ctx.Input.Args = options.args
	}
	if len(options.attachments) != 0 {
		ctx.Input.Attachments = options.attachments
	}
	e := base.NewSentinelEntry(ctx, rw, sc)
	ctx.SetEntry(e)
	r := sc.Entry(ctx) // 主逻辑
	if r == nil {
		// 这表明某些槽中存在内部错误，所以直接通过
		return e, nil
	}
	if r.Status() == base.ResultStatusBlocked {
		// r will be put to Pool in calling Exit()
		// must finish the lifecycle of r.
		blockErr := base.NewBlockErrorFromDeepCopy(r.BlockError())
		e.Exit()
		return nil, blockErr
	}

	return e, nil
}

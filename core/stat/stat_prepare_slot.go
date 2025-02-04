package stat

import (
	"github.com/alibaba/sentinel-golang/core/base"
)

const (
	PrepareSlotOrder = 1000
)

var (
	DefaultResourceNodePrepareSlot = &ResourceNodePrepareSlot{}
)

type ResourceNodePrepareSlot struct {
}

func (s *ResourceNodePrepareSlot) Order() uint32 {
	return PrepareSlotOrder
}

func (s *ResourceNodePrepareSlot) Prepare(ctx *base.EntryContext) {
	node := GetOrCreateResourceNode(ctx.Resource.Name(), ctx.Resource.Classification())
	ctx.StatNode = node
}

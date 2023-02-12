package base

import "fmt"

// ResourceType 表示资源的分类
type ResourceType int32

const (
	ResTypeCommon ResourceType = iota
	ResTypeWeb
	ResTypeRPC
	ResTypeAPIGateway
	ResTypeDBSQL
	ResTypeCache
	ResTypeMQ
)

// TrafficType 描述了流量类型:Inbound或Outbound
type TrafficType int32

const (
	Inbound TrafficType = iota
	Outbound
)

func (t TrafficType) String() string {
	switch t {
	case Inbound:
		return "Inbound"
	case Outbound:
		return "Outbound"
	default:
		return fmt.Sprintf("%d", t)
	}
}

// ResourceWrapper 表示调用
type ResourceWrapper struct {
	name           string       // 全局唯一资源名
	classification ResourceType // 资源类型
	flowType       TrafficType  // 流量类型
}

func (r *ResourceWrapper) String() string {
	return fmt.Sprintf("资源调用{name=%s, flowType=%s, classification=%d}", r.name, r.flowType, r.classification)
}

func (r *ResourceWrapper) Name() string {
	return r.name
}

func (r *ResourceWrapper) Classification() ResourceType {
	return r.classification
}

func (r *ResourceWrapper) FlowType() TrafficType {
	return r.flowType
}

func NewResourceWrapper(name string, classification ResourceType, flowType TrafficType) *ResourceWrapper {
	return &ResourceWrapper{name: name, classification: classification, flowType: flowType}
}

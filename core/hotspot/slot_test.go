package hotspot

import (
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/stretchr/testify/mock"
)

type TrafficShapingControllerMock struct {
	mock.Mock
}

func (m *TrafficShapingControllerMock) PerformChecking(arg interface{}, batchCount int64) *base.TokenResult {
	retArgs := m.Called(arg, batchCount)
	return retArgs.Get(0).(*base.TokenResult)
}

func (m *TrafficShapingControllerMock) BoundParamIndex() int {
	retArgs := m.Called()
	return retArgs.Int(0)
}

func (m *TrafficShapingControllerMock) BoundMetric() *ParamsMetric {
	retArgs := m.Called()
	return retArgs.Get(0).(*ParamsMetric)
}

func (m *TrafficShapingControllerMock) BoundRule() *Rule {
	retArgs := m.Called()
	return retArgs.Get(0).(*Rule)
}

func (m *TrafficShapingControllerMock) Replace(r *Rule) {
	_ = m.Called(r)
	return
}

func (m *TrafficShapingControllerMock) ExtractArgs(ctx *base.EntryContext) []interface{} {
	_ = m.Called()
	ret := []interface{}{ctx.Input.Args[m.BoundParamIndex()]}
	return ret
}

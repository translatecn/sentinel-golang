package flow

import (
	"log"
	"math"
	"testing"

	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/alibaba/sentinel-golang/tests/benchmark"
)

const (
	resReject = "abc-reject"
	resWarmUp = "abc-warmup"
)

func doCheck(res string) {
	if se, err := sentinel.Entry(res); err == nil {
		se.Exit()
	} else {
		log.Fatalf("Block err: %s", err.Error())
	}
}

func init() {
	benchmark.InitSentinel()
	rule1 := &flow.Rule{
		Resource:               resReject,
		TokenCalculateStrategy: flow.Constant,
		ControlBehavior:        flow.Reject,
		Threshold:              math.MaxFloat64,
		StatIntervalInMs:       1000,
		RelationStrategy:       flow.CurrentResource,
	}
	rule2 := &flow.Rule{
		Resource:               resWarmUp,
		TokenCalculateStrategy: flow.WarmUp,
		ControlBehavior:        flow.Reject,
		Threshold:              math.MaxFloat64,
		WarmUpPeriodSec:        10,
		WarmUpColdFactor:       3,
		StatIntervalInMs:       1000,
	}
	_, err := flow.LoadRules([]*flow.Rule{rule1, rule2})
	if err != nil {
		panic(err)
	}
}

func Benchmark_DirectReject_SlotCheck_4(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.SetParallelism(4)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			doCheck(resReject)
		}
	})
}

func Benchmark_DirectReject_SlotCheck_8(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.SetParallelism(8)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			doCheck(resReject)
		}
	})
}

func Benchmark_DirectReject_SlotCheck_16(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.SetParallelism(16)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			doCheck(resReject)
		}
	})
}

func Benchmark_DirectReject_SlotCheck_32(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.SetParallelism(32)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			doCheck(resReject)
		}
	})
}

func Benchmark_WarmUpReject_SlotCheck_4(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.SetParallelism(4)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			doCheck(resReject)
		}
	})
}

func Benchmark_WarmUpReject_SlotCheck_8(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.SetParallelism(8)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			doCheck(resWarmUp)
		}
	})
}

func Benchmark_WarmUpReject_SlotCheck_16(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.SetParallelism(16)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			doCheck(resWarmUp)
		}
	})
}

func Benchmark_WarmUpReject_SlotCheck_32(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.SetParallelism(32)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			doCheck(resWarmUp)
		}
	})
}

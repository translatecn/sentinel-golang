package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/core/config"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/alibaba/sentinel-golang/core/system_metric"
	"github.com/alibaba/sentinel-golang/logging"
)

const resName = "example-memory-adaptive-qps-flow-resource"

func main() {
	conf := config.NewDefaultConfig()
	conf.Sentinel.Log.Logger = logging.NewConsoleLogger()
	// 使用模拟内存使用代替实际内存使用，因此关闭内存收集器
	conf.Sentinel.Stat.System.CollectIntervalMs = 0
	conf.Sentinel.Stat.System.CollectMemoryIntervalMs = 0

	err := sentinel.InitWithConfig(conf)
	if err != nil {
		log.Fatal(err)
	}

	_, err = flow.LoadRules([]*flow.Rule{
		{
			Resource:               resName,
			TokenCalculateStrategy: flow.MemoryAdaptive,
			ControlBehavior:        flow.Reject,
			StatIntervalInMs:       1000,
			LowMemUsageThreshold:   1000,
			HighMemUsageThreshold:  100,
			MemLowWaterMarkBytes:   1024,
			MemHighWaterMarkBytes:  2048,
		},
	})
	if err != nil {
		log.Fatalf("Unexpected error: %+v", err)
		return
	}

	// 模拟内存使用是1000字节，所以QPS阈值应该是1000
	system_metric.SetSystemMemoryUsage(999)
	ch := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			for {
				e, b := sentinel.Entry(resName, sentinel.WithTrafficType(base.Inbound))
				if b != nil {
					// Blocked. We could get the block reason from the BlockError.
					time.Sleep(time.Duration(rand.Uint64()%2) * time.Millisecond)
				} else {
					// Passed, wrap the logic here.
					fmt.Println(time.Now(), "pass")
					time.Sleep(time.Duration(rand.Uint64()%2) * time.Millisecond)
					// Be sure the entry is exited finally.
					e.Exit()
				}
			}
		}()
	}

	// Simulate a scenario in which flow rules are updated concurrently
	go func() {
		time.Sleep(time.Second * 5)
		// mock memory usage is 1536 bytes, so QPS threshold should be 550
		system_metric.SetSystemMemoryUsage(1536)

		time.Sleep(time.Second * 5)
		// mock memory usage is 1536 bytes, so QPS threshold should be 100
		system_metric.SetSystemMemoryUsage(2048)

		time.Sleep(time.Second * 5)
		// mock memory usage is 1536 bytes, so QPS threshold should be 100
		system_metric.SetSystemMemoryUsage(100000)
	}()
	<-ch
}

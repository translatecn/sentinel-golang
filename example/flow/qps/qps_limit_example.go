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
	"github.com/alibaba/sentinel-golang/logging"
)

const resName = "example-flow-qps-resource"

func main() {
	conf := config.NewDefaultConfig()
	conf.Sentinel.Log.Logger = logging.NewConsoleLogger()
	err := sentinel.InitWithConfig(conf)
	if err != nil {
		log.Fatal(err)
	}

	_, err = flow.LoadRules([]*flow.Rule{
		{
			Resource:               resName,
			TokenCalculateStrategy: flow.Direct,
			ControlBehavior:        flow.Reject,
			Threshold:              1,
			StatIntervalInMs:       1000,
		},
	})
	if err != nil {
		log.Fatalf("Unexpected error: %+v", err)
		return
	}

	ch := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func() {
			for {
				e, b := sentinel.Entry(resName, sentinel.WithTrafficType(base.Inbound)) // 一个请求量
				if b != nil {
					//fmt.Println(time.Now(), b)
					// 屏蔽。我们可以从BlockError中得到阻塞原因。
					time.Sleep(time.Duration(rand.Uint64()%10) * time.Millisecond)
				} else {
					fmt.Println(time.Now(), "pass")
					//time.Sleep(time.Duration(rand.Uint64()%10) * time.Millisecond)
					e.Exit() // 确保最终退出该条目。
				}
			}
		}()
	}

	// 模拟一个并发更新流规则的场景
	go func() {
		time.Sleep(time.Second * 10)
		_, err = flow.LoadRules([]*flow.Rule{
			{
				Resource:               resName,
				TokenCalculateStrategy: flow.Direct,
				ControlBehavior:        flow.Reject,
				Threshold:              2,
				StatIntervalInMs:       100000,
			},
		})
		if err != nil {
			log.Fatalf("Unexpected error: %+v", err)
			return
		}
	}()
	<-ch
}

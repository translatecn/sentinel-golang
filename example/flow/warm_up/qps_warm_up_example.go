package main

// 预热,服务能力越来越高
import (
	"fmt"
	"log"
	"math/rand"
	"sync/atomic"
	"time"

	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/core/config"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/alibaba/sentinel-golang/logging"
	"github.com/alibaba/sentinel-golang/util"
)

type Counter struct {
	pass  *int64
	block *int64
	total *int64
}

var routineCount = 30

func main() {
	counter := Counter{pass: new(int64), block: new(int64), total: new(int64)}
	// We should initialize Sentinel first.
	conf := config.NewDefaultConfig()
	// for testing, logging output to console
	conf.Sentinel.Log.Logger = logging.NewConsoleLogger()
	err := sentinel.InitWithConfig(conf)
	if err != nil {
		log.Fatal(err)
	}

	_, err = flow.LoadRules([]*flow.Rule{
		{
			Resource:               "some-test",
			Threshold:              100,
			TokenCalculateStrategy: flow.WarmUp,
			ControlBehavior:        flow.Reject,
			WarmUpPeriodSec:        10,
			WarmUpColdFactor:       3,
			StatIntervalInMs:       1000,
		},
	})
	if err != nil {
		log.Fatalf("Unexpected error: %+v", err)
		return
	}
	go timerPrintTask(&counter)
	ch := make(chan struct{})
	//warmUp task
	for i := 0; i < 3; i++ {
		go Task(&counter)
	}
	time.Sleep(3 * time.Second)
	//sentinel task
	for i := 0; i < routineCount; i++ {
		go Task(&counter)
	}
	<-ch
}

func timerPrintTask(counter *Counter) {
	fmt.Println("begin to statistic!!!")
	var (
		oldTotal, oldPass, oldBlock int64
	)
	for {
		time.Sleep(1 * time.Second)
		globalTotal := atomic.LoadInt64(counter.total)
		oneSecondTotal := globalTotal - oldTotal
		oldTotal = globalTotal

		globalPass := atomic.LoadInt64(counter.pass)
		oneSecondPass := globalPass - oldPass
		oldPass = globalPass

		globalBlock := atomic.LoadInt64(counter.block)
		oneSecondBlock := globalBlock - oldBlock
		oldBlock = globalBlock
		fmt.Println(util.CurrentTimeMillis()/1000, "total:", oneSecondTotal, " pass:", oneSecondPass, " block:", oneSecondBlock)
	}
}
func Task(counter *Counter) {
	for {
		atomic.AddInt64(counter.total, 1)
		e, b := sentinel.Entry("some-test", sentinel.WithTrafficType(base.Inbound))
		if b != nil {
			atomic.AddInt64(counter.block, 1)
		} else {
			// Be sure the entry is exited finally.
			e.Exit()
			atomic.AddInt64(counter.pass, 1)
		}
		time.Sleep(time.Duration(rand.Uint64()%50) * time.Millisecond)
	}
}

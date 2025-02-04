package main

import (
	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/config"
	"github.com/alibaba/sentinel-golang/core/isolation"
	"github.com/alibaba/sentinel-golang/logging"
	"os"
	"time"
)

func main() {
	cfg := config.NewDefaultConfig()
	cfg.Sentinel.Log.Logger = logging.NewConsoleLogger()
	err := sentinel.InitWithConfig(cfg)
	if err != nil {
		logging.Error(err, "fail")
		os.Exit(1)
	}
	logging.ResetGlobalLoggerLevel(logging.DebugLevel)
	ch := make(chan struct{})

	r1 := &isolation.Rule{
		Resource:   "abc",
		MetricType: isolation.Concurrency,
		Threshold:  2,
	}
	_, err = isolation.LoadRules([]*isolation.Rule{r1})
	if err != nil {
		logging.Error(err, "fail")
		os.Exit(1)
	}

	for i := 0; i < 4; i++ {
		go func() {
			for {
				time.Sleep(time.Second)
				e, b := sentinel.Entry("abc", sentinel.WithBatchCount(1))
				if b != nil {
					logging.Info("[Isolation] Blocked", "reason", b.BlockType().String(), "rule", b.TriggeredRule(), "snapshot", b.TriggeredValue())
					//time.Sleep(time.Duration(rand.Uint64()%20) * time.Millisecond)
				} else {
					logging.Info("[Isolation] Passed")
					//time.Sleep(time.Duration(rand.Uint64()%20) * time.Millisecond)
					e.Exit()
				}
			}
		}()
	}
	<-ch
}

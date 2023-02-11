package main

import (
	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/gin-gonic/gin"
)

func initSentinel() {
	err := sentinel.InitDefault()
	if err != nil {
		panic(err)
	}
	_, err = flow.LoadRules([]*flow.Rule{
		{
			Resource:               "GET:/test",
			Threshold:              1.0,
			TokenCalculateStrategy: flow.Direct,
			ControlBehavior:        flow.Reject,
			StatIntervalInMs:       100000, // Threshold/StatIntervalInMs
		},
	})
	if err != nil {
		panic(err)
	}
}
func main() {
	r := gin.New()
	r.Use(
		SentinelMiddleware(
			// 默认情况下，如果需要自定义资源提取器方法路径
			WithResourceExtractor(func(ctx *gin.Context) string {
				//return ctx.GetHeader("X-Real-IP")
				return ctx.FullPath() // 请求路径
			}),
			// 自定义块回退，如果需要终止，默认状态为429
			WithBlockFallback(func(ctx *gin.Context) {
				ctx.AbortWithStatusJSON(400, map[string]interface{}{
					"err":  "too many request; the quota used up",
					"code": 10222,
				})
			}),
		),
	)
	initSentinel()

	r.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})
	_ = r.Run(":12345")
}

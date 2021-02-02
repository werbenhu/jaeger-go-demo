package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
	"io"
	"log"
	"time"
)

func initJaeger(serviceName string) io.Closer {
	cfg := &config.Configuration{
		Sampler: &config.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &config.ReporterConfig{
			LogSpans: true,
			LocalAgentHostPort:"218.91.230.20:6831",
		},
	}
	closer, err := cfg.InitGlobalTracer(serviceName, config.Logger(jaeger.StdLogger))

	if err != nil {
		panic(fmt.Sprintf("ERROR: cannot init Jaeger: %v\n", err))
	}
	return closer
}

// 这里本地span，不用跨服务器
func selfCall(ctx context.Context) context.Context {
	clientSpan, clientCtx := opentracing.StartSpanFromContext(ctx, "self-two-call")
	time.Sleep(time.Second)
	clientSpan.Finish()
	return clientCtx
}

func main() {

	//初始化jaeger
	jaeger := initJaeger("server-two")
	defer jaeger.Close()

	//初始化消费者
	InitEvent()

	r := gin.Default()

	// server-two接口，接受处理server-one发送来的http请求
	r.GET("/server_two", func(c *gin.Context) {

		// 从http头中提取trace-id
		// refer to https://github.com/opentracing/opentracing-go
		// refer to https://opentracing.io/docs/overview/inject-extract/
		carrier := opentracing.HTTPHeadersCarrier{}
		carrier.Set("uber-trace-id", c.GetHeader("uber-trace-id"))

		tracer := opentracing.GlobalTracer()
		wireContext, err := tracer.Extract(
			opentracing.HTTPHeaders,
			opentracing.HTTPHeadersCarrier(c.Request.Header))
		if err != nil {
			log.Fatal(err)
		}

		// 由传递过来的trace-id作为父span
		serverSpan := opentracing.StartSpan(
			"server-two-http-root",
			ext.RPCServerOption(wireContext))

		ctx := opentracing.ContextWithSpan(context.Background(), serverSpan)
		selfCall(ctx)

		c.JSON(200, gin.H{
			"message": "server two response",
		})
		serverSpan.Finish()
	})
	r.Run("0.0.0.0:9002")
}

//
//  @File : main.go.go
//	@Author : WerBen
//  @Email : 289594665@qq.com
//	@Time : 2021/2/1 17:19 
//	@Desc : TODO ...
//

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)
const (
	TopicName string = "test"
	TopicCh string = "test"
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

// http传递trace-id
func twoCall(ctx context.Context) context.Context {

	// 通过http向server-two请求数据
	client := &http.Client{}
	req, _ := http.NewRequest("GET","http://localhost:9002/server_two",nil)

	// refer to https://github.com/opentracing/opentracing-go
	// refer to https://opentracing.io/docs/overview/inject-extract/
	tracer := opentracing.GlobalTracer()
	// 生成一个请求的span
	clientSpan, clientCtx := opentracing.StartSpanFromContext(ctx, "http-one-req")
	carrier := opentracing.HTTPHeadersCarrier{}
	tracer.Inject(clientSpan.Context(), opentracing.HTTPHeaders, carrier)

	// 将当前span的trace-id传递到http header中
	for key, value := range carrier {
		req.Header.Add(key, value[0])
	}

	// 发送请求
	resp, _ := client.Do(req)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	//结束当前请求的span
	clientSpan.Finish()
	fmt.Printf(string(body))
	return clientCtx
}

// nsq消息队列传递trace-id，其他的如kafka等MQ也可以类似
func nsqCall(ctx context.Context) context.Context {

	tracer := opentracing.GlobalTracer()
	clientSpan, clientCtx := opentracing.StartSpanFromContext(ctx, "nsq-one-req")
	carrier := opentracing.HTTPHeadersCarrier{}
	tracer.Inject(clientSpan.Context(), opentracing.HTTPHeaders, carrier)

	// 将trace-id封装到消息中，由消息队列，传给消费者
	msg, _ := json.Marshal(carrier)
	// 生产消息
	Produce(TopicName, string(msg))

	clientSpan.Finish()
	return clientCtx
}

// 这里本地span，不用跨服务器
func selfCall(ctx context.Context) context.Context {
	clientSpan, clientCtx := opentracing.StartSpanFromContext(ctx, "self-one-call")
	time.Sleep(time.Second)
	clientSpan.Finish()
	return clientCtx
}

func main() {

	//初始化jaeger
	jaeger := initJaeger("server-one")
	defer jaeger.Close()
	r := gin.Default()

	// server-one接口，收到请求会给server-two发送http请求
	r.GET("/req_one", func(c *gin.Context) {
		span, ctx := opentracing.StartSpanFromContext(context.Background(), "server-one-http-root")
		ctx, cancel := context.WithTimeout(ctx, 10 * time.Second)
		defer cancel()

		newCtx := twoCall(ctx)
		newCtx = selfCall(newCtx)

		time.Sleep(2 * time.Second)
		span.Finish()
		c.JSON(200, gin.H{
			"message": "hello_one response",
		})
	})

	// server-one接口，收到请求会给server-two发送消息（nsq消息队列）
	r.GET("/nsq_one", func(c *gin.Context) {
		span, ctx := opentracing.StartSpanFromContext(context.Background(), "nsq-one-root")
		ctx, cancel := context.WithTimeout(ctx, 10 * time.Second)
		defer cancel()

		newCtx := nsqCall(ctx)
		newCtx = selfCall(newCtx)

		time.Sleep(2 * time.Second)
		span.Finish()
		c.JSON(200, gin.H{
			"message": "hello_one response",
		})
	})
	r.Run("0.0.0.0:9001")
}
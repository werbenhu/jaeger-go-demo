package main

/*
@Time : 2020/09/04
@Author : WerBen
@File : event.go
@Desc : dev's event via nsq
*/

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"log"
)

const (
	TopicName  string = "test"
	TopicCh string = "test"
)

// 消费者消息处理，处理server-one发送过来的消息
func eventHandler(message string) error {
	fmt.Printf("event msg:%s\n", message)

	jaeger := initJaeger("server-two")
	defer jaeger.Close()

	// 读取消息中的trace-id
	var carrier opentracing.HTTPHeadersCarrier
	json.Unmarshal([]byte(message), &carrier)

	tracer := opentracing.GlobalTracer()
	wireContext, err := tracer.Extract(
		opentracing.HTTPHeaders,
		carrier)
	if err != nil {
		log.Fatal(err)
	}

	// 由传递过来的trace-id作为父span
	serverSpan := opentracing.StartSpan(
		"nsq_two",
		ext.RPCServerOption(wireContext))

	ctx := opentracing.ContextWithSpan(context.Background(), serverSpan)
	selfCall(ctx)
	serverSpan.Finish()

	return nil
}

func InitEvent() {
	Consume(TopicName, TopicCh, eventHandler)
}

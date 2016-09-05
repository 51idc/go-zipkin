[![Build Status](https://travis-ci.org/51idc/go-zipkin.svg?branch=master)](https://travis-ci.org/51idc/go-zipkin)

# Zipkin library for Go

This library focus is to provide maximum flexibility for creating [Zipkin](http://zipkin.io) traces inside the distributed application.
The library presumes the user is familiar with basic Zipkin concepts like annotations, spans, services, etc.
Zipkin spans are composed in a stateless fashion. It is up to you how to manage span entities inside the application. 
Kafka is used as a transport to transfer the completed spans to Zipkin collector.

## Quickstart
 
```go
package main

import (
	"github.com/51idc/go-zipkin"
)

func main() {

	// tracing rate 1 of 10
	// 1 is 100%
	rate := 1
	brokerAddr := []string{"127.0.0.1:9092"} // Kafka broker endpoint

	tracer := zipkin.NewTracer(&zipkin.Option{
		ServiceName: "Zipkin Test",
		Rate:        rate,
		BrokerList:  brokerAddr,
		IP:          zipkin.LocalNetworkIP(),
		Topic:       zipkin.DefaultTopic(),
	})

	span := tracer.NewSpan("span_name")
	span.ServerReceive()

	// do work here

	span.ServerSendAndCollect()
}

```

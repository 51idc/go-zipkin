package zipkin

import (
	"fmt"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func TestTracer(t *testing.T) {

	tracer := NewTracer(&Option{
		ServiceName: "Zipkin Test",
		Rate:        1, //  100%
		BrokerList:  []string{"127.0.0.1:9092"},
		IP:          LocalNetworkIP(),
		Topic:       "zipkin",
	})

	span := tracer.NewSpan("DemoMethod")

	span.ClientSend()
	demoMethod() // Do work
	span.ClientReceiveAndCollect()

	//span.ClientReceiveAndCollect()
	//time.Sleep(1 * time.Second)
}

func demoMethod() {
	time.Sleep(2 * time.Second)
	fmt.Println("demoMethod Finished")
}

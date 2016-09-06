package zipkin

import (
	"sync"

	"github.com/Shopify/sarama"
)

type Collector interface {
	Collect([]byte)
}

type kafkaCollector struct {
	sync.Mutex
	producer sarama.SyncProducer
	topic    string
	msgCh    chan *sarama.ProducerMessage
}

func NewKafkaCollector(topic string, producer sarama.SyncProducer) *kafkaCollector {
	c := &kafkaCollector{
		producer: producer,
		topic:    topic,
		msgCh:    make(chan *sarama.ProducerMessage, 10),
	}

	go func() {
		for m := range c.msgCh {
			c.producer.SendMessage(m)
		}
	}()

	return c
}

func (c *kafkaCollector) Collect(bytes []byte) {
	c.msgCh <- &sarama.ProducerMessage{
		Topic: c.topic,
		Value: sarama.ByteEncoder(bytes),
	}
}

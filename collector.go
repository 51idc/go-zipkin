package zipkin

import (
	"github.com/Shopify/sarama"
	log "github.com/Sirupsen/logrus"
)

type Collector interface {
	Collect([]byte)
}

type KafkaCollector struct {
	producer sarama.SyncProducer
	topic    string
}

func (kc *KafkaCollector) Collect(bytes []byte) {
	log.Debugf("[Zipkin] Collecting bytes: %v", bytes)
	kc.producer.SendMessage(&sarama.ProducerMessage{Topic: kc.topic, Value: sarama.ByteEncoder(bytes)})
	log.Debugf("[Zipkin] Bytes collected")
}

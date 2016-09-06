package zipkin

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/51idc/go-zipkin/gen-go/zipkincore"
	"github.com/Shopify/sarama"
	log "github.com/Sirupsen/logrus"
	"github.com/elodina/go-avro"
)

var localhost int32 = 127*256*256*256 + 1

type Tracer struct {
	sync.Mutex
	collector   Collector
	ip          int32
	port        int16
	serviceName string
	rate        int
}

type Option struct {
	ServiceName string
	Rate        int
	BrokerList  []string // Options if Collector exists
	Collector   Collector
	IP          string
	Port        int16
	Topic       string
}

func NewTracer(opt *Option) *Tracer {
	log.Infof(
		"[Zipkin] Creating new tracer for service %s with rate 1:%d, topic %s, ip %s, port %d",
		opt.ServiceName, opt.Rate, opt.Topic, opt.IP, opt.Port,
	)

	collector := opt.Collector
	if collector == nil {
		if producer, err := DefaultProducer(opt.BrokerList); err == nil {
			collector = NewKafkaCollector(opt.Topic, producer)
		}
	}

	convertedIp, err := convertIp(opt.IP)
	if err != nil {
		log.Warningf("Given ip %s is not a valid ipv4 ip address, going with localhost ip")
		convertedIp = &localhost
	}

	tracer := &Tracer{
		ip:          *convertedIp,
		port:        opt.Port,
		collector:   collector,
		rate:        opt.Rate,
		serviceName: opt.ServiceName,
	}
	return tracer
}

func (t *Tracer) SetCollector(c Collector) {
	t.Lock()
	t.collector = c
	t.Unlock()
}

func convertIp(ip string) (*int32, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil, errors.New("Unable to parse given ip")
	}

	var ipInInt int32
	buf := bytes.NewReader([]byte{parsedIP[0], parsedIP[1], parsedIP[2], parsedIP[3]})
	err := binary.Read(buf, binary.LittleEndian, &ipInInt)
	if err == nil {
		return &ipInInt, nil
	} else {
		return nil, err
	}
}

func DefaultTopic() string {
	return "zipkin"
}

func DefaultPort() int16 {
	return 0
}

func DefaultProducer(brokerList []string) (sarama.SyncProducer, error) {
	c := sarama.NewConfig()
	c.Producer.Retry.Max = 3
	c.Producer.RequiredAcks = sarama.WaitForAll
	c.ClientID = "zipkin"

	return sarama.NewSyncProducer(brokerList, c)

	// producerConfig := producer.NewProducerConfig()
	// producerConfig.BatchSize = 200
	// producerConfig.ClientID = "zipkin"
	// kafkaConnectorConfig := siesta.NewConnectorConfig()
	// kafkaConnectorConfig.BrokerList = brokerList
	// connector, err := siesta.NewDefaultConnector(kafkaConnectorConfig)
	// if err != nil {
	// 	return nil, err
	// }
	// return producer.NewKafkaProducer(producerConfig, producer.ByteSerializer, producer.ByteSerializer, connector), nil
}

func LocalNetworkIP() string {
	ip, err := determineLocalIp()
	if err == nil {
		return ip
	} else {
		log.Warningf("Unable to determine local network IP address, going with localhost IP")
		return "127.0.0.1"
	}
}

func determineLocalIp() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("Not connected to network")
}

func (t *Tracer) NewSpan(name string) *Span {
	log.Debugf("[Zipkin] Creating new span: %s", name)
	sampled := rand.Intn(t.rate) == 0
	if !sampled {
		return &Span{sampled: false}
	}
	span := newSpan(name, newID(), newID(), nil, t.serviceName)
	span.collector = t.collector
	span.port = t.port
	span.ip = t.ip
	span.sampled = true
	return span
}

type Span struct {
	sync.Mutex
	Span        *zipkincore.Span
	collector   Collector
	ip          int32
	port        int16
	sampled     bool
	serviceName string
}

func newSpan(name string, traceID int64, spanId int64, parentID *int64, serviceName string) *Span {
	zipkinSpan := &zipkincore.Span{
		Name:              name,
		ID:                spanId,
		TraceID:           traceID,
		ParentID:          parentID,
		Annotations:       make([]*zipkincore.Annotation, 0),
		BinaryAnnotations: make([]*zipkincore.BinaryAnnotation, 0),
	}

	return &Span{Span: zipkinSpan, serviceName: serviceName}
}

func (s *Span) Sampled() bool {
	return s.sampled
}

func (s *Span) TraceID() int64 {
	return s.Span.TraceID
}

func (s *Span) ParentID() *int64 {
	return s.Span.ParentID
}

func (s *Span) ID() int64 {
	return s.Span.ID
}

func (s *Span) SetParentID(pid int64) {
	s.Span.ParentID = &pid
}

func (s *Span) SetID(id int64) {
	s.Span.ID = id
}

func (s *Span) ServerReceive() {
	log.Debugf("[Zipkin] ServerReceive")
	s.Annotate(zipkincore.SERVER_RECV)
}

func (s *Span) ServerSend() {
	log.Debugf("[Zipkin] ServerSend")
	s.Annotate(zipkincore.SERVER_SEND)
}

func (s *Span) ServerSendAndCollect() {
	s.ServerSend()
	s.Collect()
}

func (s *Span) ServerReceiveAndCollect() {
	s.ServerReceive()
	s.Collect()
}

func (s *Span) ClientSend() {
	log.Debugf("[Zipkin] ClientSend")
	s.Annotate(zipkincore.CLIENT_SEND)
}

func (s *Span) ClientReceive() {
	log.Debugf("[Zipkin] ClientReceive")
	s.Annotate(zipkincore.CLIENT_RECV)
}

func (s *Span) ClientReceiveAndCollect() {
	s.ClientReceive()
	s.Collect()
}

func (s *Span) ClientSendAndCollect() {
	s.ClientSend()
	s.Collect()
}

func (s *Span) NewChild(name string) *Span {
	log.Debugf("[Zipkin] Creating new child span: %s", name)
	if !s.sampled {
		return &Span{}
	}
	child := newSpan(name, s.Span.TraceID, newID(), &s.Span.ID, s.serviceName)
	child.collector = s.collector
	child.ip = s.ip
	child.port = s.port
	child.sampled = true
	return child
}

func (t *Tracer) NewSpanFromAvro(name string, traceInfo interface{}) *Span {

	if traceInfo == nil {
		return t.NewSpanFromRequest(name, nil, nil, nil, nil)
	}

	traceInfoDef := traceInfo.(*avro.GenericRecord)

	var traceId *int64 = nil
	if traceIdAvro := traceInfoDef.Get("traceId"); traceIdAvro != nil {
		traceIdDef := traceIdAvro.(int64)
		traceId = &traceIdDef
	}

	var spanId *int64 = nil
	if spanIdAvro := traceInfoDef.Get("spanId"); spanIdAvro != nil {
		spanIdDef := spanIdAvro.(int64)
		spanId = &spanIdDef
	}

	var sampled *bool = nil
	if sampledAvro := traceInfoDef.Get("sampled"); sampledAvro != nil {
		sampledDef := sampledAvro.(bool)
		sampled = &sampledDef
	}

	var parentId *int64 = nil
	if parentIdAvro := traceInfoDef.Get("parentSpanId"); parentIdAvro != nil {
		parentIdDef := parentIdAvro.(int64)
		parentId = &parentIdDef
	}

	return t.NewSpanFromRequest(name, traceId, spanId, parentId, sampled)
}

func (t *Tracer) NewSpanFromRequest(name string, traceId *int64, spanId *int64, parentId *int64, sampled *bool) *Span {
	nonSampled := &Span{sampled: false}
	if sampled == nil {
		log.Debugf("[Zipkin] Empty trace info provided. Ignoring")
		return nonSampled
	}
	if !*sampled {
		log.Debugf("[Zipkin] The input trace info not sampled. Ignoring")
		return nonSampled
	}
	if spanId == nil || traceId == nil {
		log.Debugf("[Zipkin] The input trace info incomplete. Ignoring")
		return nonSampled
	}

	log.Debugf("[Zipkin] Creating new span %s from request: traceID %s, spanID %s, parentID %s, sampled %s", name,
		traceId, spanId, parentId, sampled)
	span := newSpan(name, *traceId, *spanId, nil, t.serviceName)
	span.collector = t.collector
	span.port = t.port
	span.ip = t.ip
	span.sampled = true
	return span
}

func (s *Span) GetAvroTraceInfo() *avro.GenericRecord {
	if s.sampled {
		traceInfo := avro.NewGenericRecord(NewTraceInfo().Schema())
		traceInfo.Set("traceId", s.TraceID())
		traceInfo.Set("spanId", s.ID())
		if parentId := s.ParentID(); parentId != nil {
			traceInfo.Set("parentSpanId", *parentId)
		}
		traceInfo.Set("sampled", true)
		return traceInfo
	} else {
		return nil
	}
}

func (s *Span) Collect() error {
	if s.collector == nil{
		panic("[Zipkin] Collector not configured")
	}

	log.Debugf("[Zipkin] Sending spans: %b", s.sampled)
	if !s.sampled {
		return nil
	}
	log.Debugf("[Zipkin] Serializing span: %v", s.Span)
	bytes, err := SerializeSpan(s.Span)
	if err != nil {
		return err
	}
	s.collector.Collect(bytes)
	return nil
}

func (s *Span) Annotate(value string) {
	if !s.sampled {
		return
	}
	now := nowMicrosecond()
	annotation := &zipkincore.Annotation{
		Value:     value,
		Timestamp: *now,
		Host: &zipkincore.Endpoint{
			ServiceName: s.serviceName,
			Ipv4:        s.ip,
			Port:        s.port,
		},
	}
	s.Lock()
	s.Span.Annotations = append(s.Span.Annotations, annotation)
	s.Unlock()
}

func nowMicrosecond() *int64 {
	now := time.Now().UnixNano() / 1000
	return &now
}

func newID() int64 {
	return rand.Int63()
}

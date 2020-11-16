package p2p

import (
	"context"
	"errors"
	"github.com/golang/protobuf/proto"
	prom "github.com/prometheus/client_golang/prometheus"
	nctx "github.com/xuperchain/xupercore/kernel/network/context"
	pb "github.com/xuperchain/xupercore/kernel/network/pb"
	"time"
)

var (
	ErrHandlerError    = errors.New("handler error")
	ErrResponseNil     = errors.New("handler response is nil")
	ErrStreamSendError = errors.New("send response error")
	ErrChannelBlock    = errors.New("channel block")
)

// Subscriber is the interface for p2p message subscriber
type Subscriber interface {
	GetMessageType() pb.XuperMessage_MessageType
	Match(*pb.XuperMessage) bool
	HandleMessage(nctx.OperateCtx, *pb.XuperMessage, Stream) error
}

// Stream send p2p response message
type Stream interface {
	Send(*pb.XuperMessage) error
}

type HandleFunc func(nctx.OperateCtx, *pb.XuperMessage) (*pb.XuperMessage, error)
type Handler interface {
	Handler(nctx.OperateCtx, *pb.XuperMessage) (*pb.XuperMessage, error)
}

type SubscriberOption func(*subscriber)

func WithFrom(from string) SubscriberOption {
	return func(s *subscriber) {
		s.from = from
	}
}

func NewSubscriber(ctx nctx.DomainCtx, typ pb.XuperMessage_MessageType, v interface{}, opts ...SubscriberOption) Subscriber {
	s := &subscriber{
		ctx: ctx,
		typ: typ,
	}

	switch obj := v.(type) {
	case Handler:
		s.handler = obj
	case chan *pb.XuperMessage:
		s.channel = obj
	default:
		ctx.GetLog().Error("not handler or channel", "msgType", typ)
		return nil
	}

	if s.handler == nil && s.channel == nil {
		ctx.GetLog().Error("need handler or channel", "msgType", typ)
		return nil
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

type subscriber struct {
	ctx nctx.DomainCtx

	typ     pb.XuperMessage_MessageType // 订阅消息类型
	from    string                      // 消息来源
	channel chan *pb.XuperMessage
	handler Handler
}

var _ Subscriber = &subscriber{}

func (s *subscriber) GetMessageType() pb.XuperMessage_MessageType {
	return s.typ
}

func (s *subscriber) Match(msg *pb.XuperMessage) bool {
	if s.from != "" && s.from != msg.GetHeader().GetFrom() {
		s.ctx.GetLog().Trace("subscriber: subscriber from not match",
			"log_id", msg.GetHeader().GetLogid(), "from", s.from, "resp.from", msg.GetHeader().GetFrom(), "type", msg.GetHeader().GetType())
		return false
	}

	return true
}

func (s *subscriber) HandleMessage(ctx nctx.OperateCtx, msg *pb.XuperMessage, stream Stream) error {
	if s.handler != nil {
		resp, err := s.handler.Handler(ctx, msg)
		if err != nil {
			ctx.GetLog().Error("subscriber: call user handler error", "log_id", msg.GetHeader().GetLogid(), "err", err)
			return ErrHandlerError
		}

		if resp == nil {
			ctx.GetLog().Error("subscriber: handler response is nil", "log_id", msg.GetHeader().GetLogid())
			return ErrResponseNil
		}

		resp.Header.Logid = msg.Header.Logid
		if err := stream.Send(resp); err != nil {
			ctx.GetLog().Error("subscriber: send response error", "log_id", msg.GetHeader().GetLogid(), "err", err)
			return ErrStreamSendError
		}

		if s.ctx.GetMetricSwitch() {
			labels := prom.Labels{
				"bcname": resp.GetHeader().GetBcname(),
				"type":   resp.GetHeader().GetType().String(),
				"method": "HandleMessage",
			}
			Metrics.Packet.With(labels).Add(float64(proto.Size(resp)))
		}
	}

	if s.channel != nil {
		timeout, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		select {
		case <-timeout.Done():
			ctx.GetLog().Error("subscriber: discard message due to channel block,", "log_id", msg.GetHeader().GetLogid(), "err", timeout.Err())
			return ErrChannelBlock
		case s.channel <- msg:
		default:
		}
	}

	return nil
}

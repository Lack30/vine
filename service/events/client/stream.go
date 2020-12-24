// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"encoding/json"
	"time"

	pb "github.com/lack-io/vine/proto/events"
	"github.com/lack-io/vine/service/client"
	"github.com/lack-io/vine/service/context"
	"github.com/lack-io/vine/service/events"
	"github.com/lack-io/vine/service/events/util"
	log "github.com/lack-io/vine/service/logger"
)

// NewStream returns an initialized stream service
func NewStream() events.Stream {
	return new(stream)
}

type stream struct {
	Client pb.StreamService
}

func (s *stream) Publish(topic string, msg interface{}, opts ...events.PublishOption) error {
	// parse the options
	options := events.PublishOptions{
		Timestamp: time.Now(),
	}
	for _, o := range opts {
		o(&options)
	}

	// encode the message if it's not already encoded
	var payload []byte
	if p, ok := msg.([]byte); ok {
		payload = p
	} else {
		p, err := json.Marshal(msg)
		if err != nil {
			return events.ErrEncodingMessage
		}
		payload = p
	}

	// execute the RPC
	_, err := s.client().Publish(context.DefaultContext, &pb.PublishRequest{
		Topic:     topic,
		Payload:   payload,
		Metadata:  options.Metadata,
		Timestamp: options.Timestamp.Unix(),
	}, client.WithAuthToken())

	return err
}

func (s *stream) Consume(topic string, opts ...events.ConsumeOption) (<-chan events.Event, error) {
	// parse options
	options := events.ConsumeOptions{AutoAck: true}
	for _, o := range opts {
		o(&options)
	}

	subReq := &pb.ConsumeRequest{
		Topic:      topic,
		Group:      options.Group,
		Offset:     options.Offset.Unix(),
		AutoAck:    options.AutoAck,
		AckWait:    options.AckWait.Nanoseconds(),
		RetryLimit: int64(options.GetRetryLimit()),
	}

	// start the stream
	stream, err := s.client().Consume(context.DefaultContext, subReq, client.WithAuthToken())
	if err != nil {
		return nil, err
	}
	evChan := make(chan events.Event)
	go func() {
		for {

			ev, err := stream.Recv()
			if err != nil {
				log.Errorf("Error receiving from stream %s", err)
				close(evChan)
				return
			}
			evt := util.DeserializeEvent(ev)
			if !options.AutoAck {
				evt.SetNackFunc(func() error {
					return stream.SendMsg(&pb.AckRequest{Id: evt.ID, Success: false})
				})
				evt.SetAckFunc(func() error {
					return stream.SendMsg(&pb.AckRequest{Id: evt.ID, Success: true})
				})
			}
			evChan <- evt
		}
	}()

	return evChan, nil
}

// this is a tmp solution since the client isn't initialized when NewStream is called. There is a
// fix in the works in another PR.
func (s *stream) client() pb.StreamService {
	if s.Client == nil {
		s.Client = pb.NewStreamService("events", client.DefaultClient)
	}
	return s.Client
}

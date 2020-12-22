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

package handler

import (
	"context"
	"time"

	"github.com/lack-io/cli"

	authns "github.com/lack-io/vine/internal/auth/namespace"
	pb "github.com/lack-io/vine/proto/broker"
	"github.com/lack-io/vine/service"
	"github.com/lack-io/vine/service/auth"
	"github.com/lack-io/vine/service/broker"
	"github.com/lack-io/vine/service/context/metadata"
	"github.com/lack-io/vine/service/errors"
	"github.com/lack-io/vine/service/logger"
	log "github.com/lack-io/vine/service/logger"
)

var (
	name    = "broker"
	address = ":8003"
)

// Run the vine broker
func Run(ctx *cli.Context) error {
	srvOpts := []service.Option{
		service.Name(name),
		service.Address(address),
	}

	if i := time.Duration(ctx.Int("register-ttl")); i > 0 {
		srvOpts = append(srvOpts, service.RegisterTTL(i*time.Second))
	}
	if i := time.Duration(ctx.Int("register-interval")); i > 0 {
		srvOpts = append(srvOpts, service.RegisterInterval(i*time.Second))
	}

	// new service
	srv := service.New(srvOpts...)

	// connect to the broker
	broker.DefaultBroker.Connect()

	// register the broker handler
	pb.RegisterBrokerHandler(srv.Server(), new(handler))

	// run the service
	if err := srv.Run(); err != nil {
		logger.Fatal(err)
	}
	return nil
}

type handler struct{}

func (h *handler) Publish(ctx context.Context, req *pb.PublishRequest, rsp *pb.Empty) error {
	// authorize the request
	acc, ok := auth.AccountFromContext(ctx)
	if !ok {
		return errors.Unauthorized("broker.Broker.Publish", authns.ErrForbidden.Error())
	}

	// validate the request
	if req.Message == nil {
		return errors.BadRequest("broker.Broker.Publish", "Missing message")
	}

	// ensure the header is not nil
	if req.Message.Header == nil {
		req.Message.Header = map[string]string{}
	}

	// set any headers which aren't already set
	if md, ok := metadata.FromContext(ctx); ok {
		for k, v := range md {
			if _, ok := req.Message.Header[k]; !ok {
				req.Message.Header[k] = v
			}
		}
	}

	log.Debugf("Publishing message to %s topic in the %v namespace", req.Topic, acc.Issuer)
	err := broker.DefaultBroker.Publish(acc.Issuer+"."+req.Topic, &broker.Message{
		Header: req.Message.Header,
		Body:   req.Message.Body,
	})
	log.Debugf("Published message to %s topic in the %v namespace", req.Topic, acc.Issuer)
	if err != nil {
		return errors.InternalServerError("broker.Broker.Publish", err.Error())
	}
	return nil
}

func (h *handler) Subscribe(ctx context.Context, req *pb.SubscribeRequest, stream pb.Broker_SubscribeStream) error {
	// authorize the request
	acc, ok := auth.AccountFromContext(ctx)
	if !ok {
		return errors.Unauthorized("broker.Broker.Subscribe", authns.ErrForbidden.Error())
	}
	ns := acc.Issuer

	errChan := make(chan error, 1)

	// message handler to stream back messages from broker
	handler := func(m *broker.Message) error {
		if err := stream.Send(&pb.Message{
			Header: m.Header,
			Body:   m.Body,
		}); err != nil {
			select {
			case errChan <- err:
				return err
			default:
				return err
			}
		}
		return nil
	}

	log.Debugf("Subscribing to %s topic in namespace %v", req.Topic, ns)
	sub, err := broker.DefaultBroker.Subscribe(ns+"."+req.Topic, handler, broker.Queue(ns+"."+req.Queue))
	if err != nil {
		return errors.InternalServerError("broker.Broker.Subscribe", err.Error())
	}
	defer func() {
		log.Debugf("Unsubscribing from topic %s in namespace %v", req.Topic, ns)
		sub.Unsubscribe()
	}()

	select {
	case <-ctx.Done():
		log.Debugf("Context done for subscription to topic %s", req.Topic)
		return nil
	case err := <-errChan:
		log.Debugf("Subscription error for topic %s: %v", req.Topic, err)
		return err
	}
}

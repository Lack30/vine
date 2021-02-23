// Copyright 2020 lack
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mucp

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/lack-io/vine/proto/apis/errors"
	regpb "github.com/lack-io/vine/proto/apis/registry"
	"github.com/lack-io/vine/service/broker"
	"github.com/lack-io/vine/service/client"
	"github.com/lack-io/vine/service/client/selector"
	"github.com/lack-io/vine/service/codec"
	raw "github.com/lack-io/vine/service/codec/bytes"
	"github.com/lack-io/vine/service/network/transport"
	"github.com/lack-io/vine/util/client/buf"
	"github.com/lack-io/vine/util/client/pool"
	"github.com/lack-io/vine/util/context/metadata"
	"github.com/lack-io/vine/util/net"
)

type rpcClient struct {
	once atomic.Value
	opts client.Options
	pool pool.Pool
	seq  uint64
}

func newRpcClient(opt ...client.Option) client.Client {
	opts := client.NewOptions(opt...)

	p := pool.NewPool(
		pool.Size(opts.PoolSize),
		pool.TTL(opts.PoolTTL),
		pool.Transport(opts.Transport),
	)

	rc := &rpcClient{
		opts: opts,
		pool: p,
		seq:  0,
	}
	rc.once.Store(false)

	c := client.Client(rc)

	// wrap in reverse
	for i := len(opts.Wrappers); i > 0; i-- {
		c = opts.Wrappers[i-1](c)
	}

	return c
}

func (r *rpcClient) newCodec(contentType string) (codec.NewCodec, error) {
	if c, ok := r.opts.Codecs[contentType]; ok {
		return c, nil
	}
	if cf, ok := DefaultCodecs[contentType]; ok {
		return cf, nil
	}
	return nil, fmt.Errorf("unsupported Content-Type: %s", contentType)
}

func (r *rpcClient) call(ctx context.Context, node *regpb.Node, req client.Request, resp interface{}, opts client.CallOptions) error {
	address := node.Address

	msg := &transport.Message{
		Header: make(map[string]string),
	}

	md, ok := metadata.FromContext(ctx)
	if ok {
		for k, v := range md {
			// don't copy Vine-Topic header, that used for pub/sub
			// this fix case then client uses the same context that received in subscriber
			if k == "Vine-Topic" {
				continue
			}
			msg.Header[k] = v
		}
	}

	// set timeout in nanoseconds
	msg.Header["Timeout"] = fmt.Sprintf("%d", opts.RequestTimeout)
	// set the content type for the request
	msg.Header["Content-Type"] = req.ContentType()
	// set the accept header
	msg.Header["Accept"] = req.ContentType()

	// setup old protocol
	cf := setupProtocol(msg, node)

	// no codec specified
	if cf == nil {
		var err error
		cf, err = r.newCodec(req.ContentType())
		if err != nil {
			return errors.InternalServerError("go.vine.client", err.Error())
		}
	}

	dOpts := []transport.DialOption{transport.WithStream()}

	if opts.DialTimeout >= 0 {
		dOpts = append(dOpts, transport.WithTimeout(opts.DialTimeout))
	}

	c, err := r.pool.Get(address, dOpts...)
	if err != nil {
		return errors.InternalServerError("go.vine.client", "connection error: %v", err)
	}

	seq := atomic.AddUint64(&r.seq, 1) - 1
	codec := newRpcCodec(msg, c, cf, "")

	rsp := &rpcResponse{
		socket: c,
		codec:  codec,
	}

	stream := &rpcStream{
		id:       fmt.Sprintf("%v", seq),
		ctx:      ctx,
		request:  req,
		response: rsp,
		codec:    codec,
		closed:   make(chan bool),
		release:  func(err error) { r.pool.Release(c, err) },
		sendEOS:  false,
	}
	// close the stream on exiting this function
	defer stream.Close()

	// wait for error response
	ch := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- errors.InternalServerError("go.vine.client", "panic recovered: %v", r)
			}
		}()

		// send request
		if err := stream.Send(req.Body()); err != nil {
			ch <- err
			return
		}

		// recv request
		if err := stream.Recv(resp); err != nil {
			ch <- err
			return
		}

		// success
		ch <- nil
	}()

	var grr error

	select {
	case err := <-ch:
		return err
	case <-ctx.Done():
		grr = errors.Timeout("go.vine.client", fmt.Sprintf("%v", ctx.Err()))
	}

	// set the stream error
	if grr != nil {
		stream.Lock()
		stream.err = grr
		stream.Unlock()

		return grr
	}

	return nil
}

func (r *rpcClient) stream(ctx context.Context, node *regpb.Node, req client.Request, opts client.CallOptions) (client.Stream, error) {
	address := node.Address

	msg := &transport.Message{
		Header: make(map[string]string),
	}

	md, ok := metadata.FromContext(ctx)
	if ok {
		for k, v := range md {
			msg.Header[k] = v
		}
	}

	// set timeout in nanoseconds
	if opts.StreamTimeout > time.Duration(0) {
		msg.Header["Timeout"] = fmt.Sprintf("%d", opts.StreamTimeout)
	}
	// set the content type for the request
	msg.Header["Content-Type"] = req.ContentType()
	// set the accept header
	msg.Header["Accept"] = req.ContentType()

	// set old codecs
	cf := setupProtocol(msg, node)

	// no codec specified
	if cf == nil {
		var err error
		cf, err = r.newCodec(req.ContentType())
		if err != nil {
			return nil, errors.InternalServerError("go.vine.client", err.Error())
		}
	}

	dOpts := []transport.DialOption{transport.WithStream()}

	if opts.DialTimeout >= 0 {
		dOpts = append(dOpts, transport.WithTimeout(opts.DialTimeout))
	}

	c, err := r.opts.Transport.Dial(address, dOpts...)
	if err != nil {
		return nil, errors.InternalServerError("go.vine.client", "connection error: %v", err)
	}

	// increment the sequence number
	seq := atomic.AddUint64(&r.seq, 1) - 1
	id := fmt.Sprintf("%v", seq)

	// create codec with stream id
	codec := newRpcCodec(msg, c, cf, id)

	rsp := &rpcResponse{
		socket: c,
		codec:  codec,
	}

	// set request codec
	if r, ok := req.(*rpcRequest); ok {
		r.codec = codec
	}

	stream := &rpcStream{
		id:       id,
		ctx:      ctx,
		request:  req,
		response: rsp,
		codec:    codec,
		// used to close the stream
		closed: make(chan bool),
		// signal the end of stream,
		sendEOS: true,
		// release func
		release: func(err error) { c.Close() },
	}

	// wait for error response
	ch := make(chan error, 1)

	go func() {
		// send the first message
		ch <- stream.Send(req.Body())
	}()

	var grr error

	select {
	case err := <-ch:
		grr = err
	case <-ctx.Done():
		grr = errors.Timeout("go.vine.client", fmt.Sprintf("%v", ctx.Err()))
	}

	if grr != nil {
		// set the error
		stream.Lock()
		stream.err = grr
		stream.Unlock()

		// close the stream
		stream.Close()
		return nil, grr
	}

	return stream, nil
}

func (r *rpcClient) Init(opts ...client.Option) error {
	size := r.opts.PoolSize
	ttl := r.opts.PoolTTL
	tr := r.opts.Transport

	for _, o := range opts {
		o(&r.opts)
	}

	// update pool configuration if the options changed
	if size != r.opts.PoolSize || ttl != r.opts.PoolTTL || tr != r.opts.Transport {
		// close existing pool
		r.pool.Close()
		// create new pool
		r.pool = pool.NewPool(
			pool.Size(r.opts.PoolSize),
			pool.TTL(r.opts.PoolTTL),
			pool.Transport(r.opts.Transport),
		)
	}

	return nil
}

func (r *rpcClient) Options() client.Options {
	return r.opts
}

// next returns an iterator for the next nodes to call
func (r *rpcClient) next(request client.Request, opts client.CallOptions) (selector.Next, error) {
	// try get the proxy
	service, address, _ := net.Proxy(request.Service(), opts.Address)

	// return remote address
	if len(address) > 0 {
		nodes := make([]*regpb.Node, len(address))

		for i, addr := range address {
			nodes[i] = &regpb.Node{
				Address: addr,
				// Set the protocol
				Metadata: map[string]string{
					"protocol": "mucp",
				},
			}
		}

		// crude return method
		return func() (*regpb.Node, error) {
			return nodes[time.Now().Unix()%int64(len(nodes))], nil
		}, nil
	}

	// get next nodes from the selector
	next, err := r.opts.Selector.Select(service, opts.SelectOptions...)
	if err != nil {
		if err == selector.ErrNotFound {
			return nil, errors.InternalServerError("go.vine.client", "service %s: %s", service, err.Error())
		}
		return nil, errors.InternalServerError("go.vine.client", "error selecting %s node: %s", service, err.Error())
	}

	return next, nil
}

func (r *rpcClient) Call(ctx context.Context, request client.Request, response interface{}, opts ...client.CallOption) error {
	// make a copy of call opts
	callOpts := r.opts.CallOptions
	for _, opt := range opts {
		opt(&callOpts)
	}

	next, err := r.next(request, callOpts)
	if err != nil {
		return err
	}

	// check if we already have a deadline
	d, ok := ctx.Deadline()
	if !ok {
		// no deadline so we create a new one
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOpts.RequestTimeout)
		defer cancel()
	} else {
		// got a deadline so no need to setup context
		// but we need to set the timeout we pass along
		opt := client.WithRequestTimeout(d.Sub(time.Now()))
		opt(&callOpts)
	}

	// should we noop right here?
	select {
	case <-ctx.Done():
		return errors.Timeout("go.vine.client", fmt.Sprintf("%v", ctx.Err()))
	default:
	}

	// make copy of call method
	rcall := r.call

	// wrap the call in reverse
	for i := len(callOpts.CallWrappers); i > 0; i-- {
		rcall = callOpts.CallWrappers[i-1](rcall)
	}

	// return errors.New("go.vine.client", "request timeout", 408)
	call := func(i int) error {
		// call backoff first, Someone may want an initial start delay
		t, err := callOpts.Backoff(ctx, request, i)
		if err != nil {
			return errors.InternalServerError("go.vine.client", "backoff error: %v", err.Error())
		}

		// only sleep if greater than 0
		if t.Seconds() > 0 {
			time.Sleep(t)
		}

		// select next node
		node, err := next()
		service := request.Service()
		if err != nil {
			if err == selector.ErrNotFound {
				return errors.InternalServerError("go.vine.client", "service %s: %s", service, err.Error())
			}
			return errors.InternalServerError("go.vine.client", "error getting next %s node: %s", service, err.Error())
		}

		// make the call
		err = rcall(ctx, node, request, response, callOpts)
		r.opts.Selector.Mark(service, node, err)
		return err
	}

	// get the retries
	retries := callOpts.Retries

	// disable retries when using a proxy
	if _, _, ok := net.Proxy(request.Service(), callOpts.Address); ok {
		retries = 0
	}

	ch := make(chan error, retries+1)
	var gerr error

	for i := 0; i <= retries; i++ {
		go func(i int) {
			ch <- call(i)
		}(i)

		select {
		case <-ctx.Done():
			return errors.Timeout("go.vine.client", fmt.Sprintf("call timeout: %v", ctx.Err()))
		case err := <-ch:
			// if the call succeeded lets bail early
			if err == nil {
				return nil
			}

			retry, rerr := callOpts.Retry(ctx, request, i, err)
			if rerr != nil {
				return rerr
			}

			if !retry {
				return err
			}

			gerr = err
		}
	}

	return gerr
}

func (r *rpcClient) Stream(ctx context.Context, request client.Request, opts ...client.CallOption) (client.Stream, error) {
	// make a copy of call opts
	callOpts := r.opts.CallOptions
	for _, opt := range opts {
		opt(&callOpts)
	}

	next, err := r.next(request, callOpts)
	if err != nil {
		return nil, err
	}

	// should we noop right here?
	select {
	case <-ctx.Done():
		return nil, errors.Timeout("go.vine.client", fmt.Sprintf("%v", ctx.Err()))
	default:
	}

	call := func(i int) (client.Stream, error) {
		// call backoff first. Someone may want an initial start delay
		t, err := callOpts.Backoff(ctx, request, i)
		if err != nil {
			return nil, errors.InternalServerError("go.vine.client", "backoff error: %v", err.Error())
		}

		// only sleep if greater than 0
		if t.Seconds() > 0 {
			time.Sleep(t)
		}

		node, err := next()
		service := request.Service()
		if err != nil {
			if err == selector.ErrNotFound {
				return nil, errors.InternalServerError("go.vine.client", "service %s: %s", service, err.Error())
			}
			return nil, errors.InternalServerError("go.vine.client", "error getting next %s node: %s", service, err.Error())
		}

		stream, err := r.stream(ctx, node, request, callOpts)
		r.opts.Selector.Mark(service, node, err)
		return stream, err
	}

	type response struct {
		stream client.Stream
		err    error
	}

	// get the retries
	retries := callOpts.Retries

	// disable retries when using a proxy
	if _, _, ok := net.Proxy(request.Service(), callOpts.Address); ok {
		retries = 0
	}

	ch := make(chan response, retries+1)
	var grr error

	for i := 0; i <= retries; i++ {
		go func(i int) {
			s, err := call(i)
			ch <- response{s, err}
		}(i)

		select {
		case <-ctx.Done():
			return nil, errors.Timeout("go.vine.client", fmt.Sprintf("call timeout: %v", ctx.Err()))
		case rsp := <-ch:
			// if the call succeeded lets bail early
			if rsp.err == nil {
				return rsp.stream, nil
			}

			retry, rerr := callOpts.Retry(ctx, request, i, rsp.err)
			if rerr != nil {
				return nil, rerr
			}

			if !retry {
				return nil, rsp.err
			}

			grr = rsp.err
		}
	}

	return nil, grr
}

func (r *rpcClient) Publish(ctx context.Context, msg client.Message, opts ...client.PublishOption) error {
	options := client.PublishOptions{Context: context.Background()}

	for _, o := range opts {
		o(&options)
	}

	md, ok := metadata.FromContext(ctx)
	if !ok {
		md = make(map[string]string)
	}

	id := uuid.New().String()
	md["Content-Type"] = msg.ContentType()
	md["Vine-Topic"] = msg.Topic()
	md["Vine-Id"] = id

	// set the topic
	topic := msg.Topic()

	// get the exchange
	if len(options.Exchange) > 0 {
		topic = options.Exchange
	}

	// encode message body
	cf, err := r.newCodec(msg.ContentType())
	if err != nil {
		return errors.InternalServerError("go.vine.client", err.Error())
	}

	var body []byte

	// passed in raw data
	if d, ok := msg.Payload().(*raw.Frame); ok {
		body = d.Data
	} else {
		// new buffer
		b := buf.New(nil)

		if err := cf(b).Write(&codec.Message{
			Target: topic,
			Type:   codec.Event,
			Header: map[string]string{
				"Vine-Id":    id,
				"Vine-Topic": msg.Topic(),
			},
		}, msg.Payload()); err != nil {
			return errors.InternalServerError("go.vine.client", err.Error())
		}

		// set the body
		body = b.Bytes()
	}

	if !r.once.Load().(bool) {
		if err = r.opts.Broker.Connect(); err != nil {
			return errors.InternalServerError("go.vine.client", err.Error())
		}
		r.once.Store(true)
	}

	return r.opts.Broker.Publish(topic, &broker.Message{
		Header: md,
		Body:   body,
	}, broker.PublishContext(options.Context))
}

func (r *rpcClient) NewMessage(topic string, message interface{}, opts ...client.MessageOption) client.Message {
	return newMessage(topic, message, r.opts.ContentType, opts...)
}

func (r *rpcClient) NewRequest(service, method string, request interface{}, reqOpts ...client.RequestOption) client.Request {
	return newRequest(service, method, request, r.opts.ContentType, reqOpts...)
}

func (r *rpcClient) String() string {
	return "mucp"
}
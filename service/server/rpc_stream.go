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

package server

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/lack-io/vine/service/codec"
)

// Implements the Streamer interface
type rpcStream struct {
	sync.RWMutex
	id      string
	closed  bool
	err     error
	request Request
	codec   codec.Codec
	ctx     context.Context
}

func (r *rpcStream) Context() context.Context {
	return r.ctx
}

func (r *rpcStream) Request() Request {
	return r.request
}

func (r *rpcStream) Send(msg interface{}) error {
	r.Lock()
	defer r.Unlock()

	resp := codec.Message{
		Target:   r.request.Service(),
		Method:   r.request.Method(),
		Endpoint: r.request.Endpoint(),
		Id:       r.id,
		Type:     codec.Response,
	}

	if err := r.codec.Write(&resp, msg); err != nil {
		r.err = err
	}

	return nil
}

func (r *rpcStream) Recv(msg interface{}) error {
	req := new(codec.Message)
	req.Type = codec.Request

	err := r.codec.ReadHeader(req, req.Type)
	r.Lock()
	defer r.Unlock()
	if err != nil {
		// discard body
		r.codec.ReadBody(nil)
		r.err = err
		return err
	}

	// check the error
	if len(req.Error) > 0 {
		// Check the client closed the stream
		switch req.Error {
		case lastStreamResponseError.Error():
			// discard body
			r.Unlock()
			r.codec.ReadBody(nil)
			r.Lock()
			r.err = io.EOF
			return io.EOF
		default:
			return errors.New(req.Error)
		}
	}

	// we need to stay up to date with sequence numbers
	r.id = req.Id
	r.Unlock()
	err = r.codec.ReadBody(msg)
	r.Lock()
	if err != nil {
		r.err = err
		return err
	}

	return nil
}

func (r *rpcStream) Error() error {
	r.RLock()
	defer r.RUnlock()
	return r.err
}

func (r *rpcStream) Close() error {
	r.Lock()
	defer r.Unlock()
	r.closed = true
	return r.codec.Close()
}

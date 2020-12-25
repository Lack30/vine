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

package rpc

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gobwas/httphead"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	json "github.com/json-iterator/go"

	"github.com/lack-io/vine/service/api"
	"github.com/lack-io/vine/service/client"
	"github.com/lack-io/vine/service/client/selector"
	raw "github.com/lack-io/vine/service/codec/bytes"
	"github.com/lack-io/vine/service/logger"
)

// serveWebsocket will stream rpc back over websockets assuming json
func serveWebsocket(ctx context.Context, w http.ResponseWriter, r *http.Request, service *api.Service, c client.Client) {
	var op ws.OpCode

	ct := r.Header.Get("Content-Type")
	// Strip charset from Content-Type (like `application/json; charset=UTF-8`)
	if idx := strings.IndexRune(ct, ';'); idx >= 0 {
		ct = ct[:idx]
	}

	// check proto from request
	switch ct {
	case "application/json":
		op = ws.OpText
	default:
		op = ws.OpBinary
	}

	hdr := make(http.Header)
	if proto, ok := r.Header["Sec-WebSocket-Protocol"]; ok {
		for _, p := range proto {
			switch p {
			case "binary":
				hdr["Sec-WebSocket-Protocol"] = []string{"binary"}
				op = ws.OpBinary
			}
		}
	}
	payload, err := requestPayload(r)
	if err != nil {
		logger.Error(err)
		return
	}

	upgrader := ws.HTTPUpgrader{Timeout: 5 * time.Second,
		Protocol: func(proto string) bool {
			if strings.Contains(proto, "binary") {
				return true
			}
			// fallback to support all protocols now
			return true
		},
		Extension: func(httphead.Option) bool {
			// disable extensions for compatibility
			return false
		},
		Header: hdr,
	}

	conn, rw, _, err := upgrader.Upgrade(r, w)
	if err != nil {
		logger.Error(err)
		return
	}

	defer func() {
		if err := conn.Close(); err != nil {
			logger.Error(err)
			return
		}
	}()

	var request interface{}
	if !bytes.Equal(payload, []byte(`{}`)) {
		switch ct {
		case "application/json", "":
			m := json.RawMessage(payload)
			request = &m
		default:
			request = &raw.Frame{Data: payload}
		}
	}

	// we always need to set content type for message
	if ct == "" {
		ct = "application/json"
	}
	req := c.NewRequest(
		service.Name,
		service.Endpoint.Name,
		request,
		client.WithContentType(ct),
		client.StreamingRequest(),
	)

	so := selector.WithStrategy(strategy(service.Services))
	// create a new stream
	stream, err := c.Stream(ctx, req, client.WithSelectOption(so))
	if err != nil {
		logger.Error(err)
		return
	}

	if request != nil {
		if err = stream.Send(request); err != nil {
			logger.Error(err)
			return
		}
	}

	go writeLoop(rw, stream)

	rsp := stream.Response()

	// receive from stream and send to client
	for {
		select {
		case <-ctx.Done():
			return
		case <-stream.Context().Done():
			return
		default:
			// read backend response body
			buf, err := rsp.Read()
			if err != nil {
				// wants to avoid import  grpc/status.Status
				if strings.Contains(err.Error(), "context canceled") {
					return
				}
				logger.Error(err)
				return
			}

			// write the response
			if err := wsutil.WriteServerMessage(rw, op, buf); err != nil {
				logger.Error(err)
				return
			}
			if err = rw.Flush(); err != nil {
				logger.Error(err)
				return
			}
		}
	}
}

// writeLoop
func writeLoop(rw io.ReadWriter, stream client.Stream) {
	// close stream when done
	defer stream.Close()

	for {
		select {
		case <-stream.Context().Done():
			return
		default:
			buf, op, err := wsutil.ReadClientData(rw)
			if err != nil {
				if wserr, ok := err.(wsutil.ClosedError); ok {
					switch wserr.Code {
					case ws.StatusGoingAway:
						// this happens when user leave the page
						return
					case ws.StatusNormalClosure, ws.StatusNoStatusRcvd:
						// this happens when user close ws connection, or we don't get any status
						return
					}
				}
				logger.Error(err)
				return
			}
			switch op {
			default:
				// not relevant
				continue
			case ws.OpText, ws.OpBinary:
				break
			}
			// send to backend
			// default to trying json
			// if the extracted payload isn't empty lets use it
			request := &raw.Frame{Data: buf}
			if err := stream.Send(request); err != nil {
				logger.Error(err)
				return
			}
		}
	}
}

func isStream(r *http.Request, srv *api.Service) bool {
	// check if it's a web socket
	if !isWebSocket(r) {
		return false
	}
	// check if the endpoint supports streaming
	for _, service := range srv.Services {
		for _, ep := range service.Endpoints {
			// skip if it doesn't match the name
			if ep.Name != srv.Endpoint.Name {
				continue
			}
			// matched if the name
			if v := ep.Metadata["stream"]; v == "true" {
				return true
			}
		}
	}
	return false
}

func isWebSocket(r *http.Request) bool {
	contains := func(key, val string) bool {
		vv := strings.Split(r.Header.Get(key), ",")
		for _, v := range vv {
			if val == strings.ToLower(strings.TrimSpace(v)) {
				return true
			}
		}
		return false
	}

	if contains("Connection", "upgrade") && contains("Upgrade", "websocket") {
		return true
	}

	return false
}

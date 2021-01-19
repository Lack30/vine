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

// Package handler is the handler for the `vineadm debug trace` service
package trace

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/lack-io/vine/proto/debug"
	"github.com/lack-io/vine/proto/debug/trace"
	"github.com/lack-io/vine/proto/errors"
	regpb "github.com/lack-io/vine/proto/registry"
	"github.com/lack-io/vine/service/client"
	"github.com/lack-io/vine/service/config/cmd"
	log "github.com/lack-io/vine/service/logger"
	"github.com/lack-io/vine/util/ring"
)

// New initialises and returns a new trace service handler
func New(done <-chan bool, windowSize int, services func() []*regpb.Service) (*Trace, error) {
	s := &Trace{
		client:    *cmd.DefaultOptions().Client,
		snapshots: ring.New(windowSize),
		services:  services,
	}

	s.Start(done)
	return s, nil
}

// trace is the Debug.trace handler
type Trace struct {
	client client.Client

	sync.RWMutex
	// snapshots
	snapshots *ring.Buffer
	// returns a list of services
	services func() []*regpb.Service
}

// Filters out all spans that are part of a trace that hits a given service.
func filterServiceSpans(service string, snapshots []*trace.Snapshot) []*trace.Span {
	// trace id -> span id -> span
	groupByTrace := map[string]map[string]*trace.Span{}
	for _, snapshot := range snapshots {
		for _, span := range snapshot.GetSpans() {
			_, ok := groupByTrace[span.GetTrace()]
			if !ok {
				groupByTrace[span.GetTrace()] = map[string]*trace.Span{}
			}
			groupByTrace[span.GetTrace()][span.GetId()] = span
		}
	}
	ret := []*trace.Span{}
	for _, spanMap := range groupByTrace {
		spans := []*trace.Span{}
		shouldAppend := false
		for _, span := range spanMap {
			spans = append(spans, span)
			if strings.Contains(span.GetName(), service) {
				shouldAppend = true
			}
			if shouldAppend {
				ret = append(ret, spans...)
			}
		}
	}
	return ret
}

func dedupeSpans(spans []*trace.Span) []*trace.Span {
	m := map[string]*trace.Span{}
	for _, span := range spans {
		m[span.GetId()] = span
	}
	ret := []*trace.Span{}
	for _, span := range m {
		ret = append(ret, span)
	}
	return ret
}

func snapshotsToSpans(snapshots []*trace.Snapshot) []*trace.Span {
	ret := []*trace.Span{}
	for _, snapshot := range snapshots {
		ret = append(ret, snapshot.GetSpans()...)
	}
	return ret
}

// Read returns gets a snapshot of all current trace3
func (s *Trace) Read(ctx context.Context, req *trace.ReadRequest, rsp *trace.ReadResponse) error {
	allSnapshots := []*trace.Snapshot{}

	s.RLock()
	defer s.RUnlock()

	// get one snapshot by default
	numEntries := 1

	// if requested get everything
	if req.Past {
		// get all
		numEntries = -1
	}

	// get the snapshots
	entries := s.snapshots.Get(numEntries)

	// build a snap slice
	for _, entry := range entries {
		allSnapshots = append(allSnapshots, entry.Value.([]*trace.Snapshot)...)
	}

	var spans []*trace.Span

	// get the list of spans
	if req.Service == nil {
		spans = dedupeSpans(snapshotsToSpans(allSnapshots))
	} else {
		spans = filterServiceSpans(req.GetService().GetName(), allSnapshots)
	}

	// no limit return all
	if req.GetLimit() == 0 {
		rsp.Spans = spans
		return nil
	}

	// get the limit of spans
	lim := req.GetLimit()
	if lim >= int64(len(spans)) {
		lim = int64(len(spans))
	}

	// set spans
	rsp.Spans = spans[0:lim]

	return nil
}

func (s *Trace) Write(ctx context.Context, req *trace.WriteRequest, rsp *trace.WriteResponse) error {
	return errors.BadRequest("go.vine.debug.trace", "not implemented")
}

// Stream starts streaming trace
func (s *Trace) Stream(ctx context.Context, req *trace.StreamRequest, rsp trace.Trace_StreamStream) error {
	return errors.BadRequest("go.vine.debug.trace", "not implemented")
}

// Start Starts scraping other services until the provided channel is closed
func (s *Trace) Start(done <-chan bool) {
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()

		for {
			select {
			case <-done:
				return
			case <-t.C:
				// now scrape fo traces
				s.scrape()
			}
		}
	}()
}

func (s *Trace) scrape() {
	// get services
	cached := s.services()

	s.RLock()
	// Create a local copy of cached services
	services := make([]*regpb.Service, len(cached))
	copy(services, cached)
	s.RUnlock()

	// get the current snaps
	entries := s.snapshots.Get(-1)

	// build a list of span ids
	ids := make(map[string]bool)

	// build a list of span ids so we can dedupe
	for _, entry := range entries {
		for _, snap := range entry.Value.([]*trace.Snapshot) {
			for _, span := range snap.Spans {
				ids[span.Id] = true
			}
		}
	}

	// Start building the next list of snapshots
	var mtx sync.Mutex
	next := make([]*trace.Snapshot, 0)

	// Call each node of each service in goroutines
	var wg sync.WaitGroup

	protocol := s.client.String()

	for _, svc := range services {
		// Ignore nodeless and non mucp services
		if len(svc.Nodes) == 0 {
			continue
		}
		// Call every node
		for _, node := range svc.Nodes {
			if node.Metadata["protocol"] != protocol {
				continue
			}

			wg.Add(1)

			go func(st *Trace, service *regpb.Service, node *regpb.Node) {
				defer wg.Done()

				// create new context to cancel within a few seconds
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				req := s.client.NewRequest(service.Name, "Debug.Trace", &debug.TraceResponse{})
				rsp := new(debug.TraceResponse)
				if err := s.client.Call(ctx, req, rsp, client.WithAddress(node.Address)); err != nil {
					log.Errorf("Error calling %s@%s (%s)", service.Name, node.Address, err.Error())
					return
				}

				var spans []*trace.Span

				for _, v := range rsp.GetSpans() {
					// we already have the span
					if ids[v.GetId()] {
						continue
					}

					var typ trace.SpanType
					switch v.GetType() {
					case debug.SpanType_INBOUND:
						typ = trace.SpanType_INBOUND
					case debug.SpanType_OUTBOUND:
						typ = trace.SpanType_OUTBOUND
					}

					spans = append(spans, &trace.Span{
						Trace:    v.GetTrace(),
						Id:       v.GetId(),
						Parent:   v.GetParent(),
						Name:     v.GetName(),
						Started:  v.GetStarted(),
						Duration: v.GetDuration(),
						Metadata: v.GetMetadata(),
						Type:     typ,
					})
				}

				// dont create snap if theres no span
				if len(spans) == 0 {
					return
				}

				// Append the new snapshot
				snap := &trace.Snapshot{
					Service: &trace.Service{
						Name:    service.Name,
						Version: service.Version,
						Node: &trace.Node{
							Id:      node.Id,
							Address: node.Address,
						},
					},
					Spans: spans,
				}

				//timestamp := time.Now().Unix()
				// snap.Timestamp = uint64(timestamp)
				mtx.Lock()
				next = append(next, snap)
				mtx.Unlock()
			}(s, svc, node)
		}
	}
	wg.Wait()

	// don't write a blank snap
	if len(next) == 0 {
		return
	}

	// save the snaps
	s.Lock()
	s.snapshots.Put(next)
	s.Unlock()
}

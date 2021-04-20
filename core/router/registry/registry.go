// MIT License
//
// Copyright (c) 2020 Lack
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package registry

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lack-io/vine/core/registry"
	rr "github.com/lack-io/vine/core/router"
	log "github.com/lack-io/vine/lib/logger"
	regpb "github.com/lack-io/vine/proto/apis/registry"
)

var (
	// AdvertiseEventsTick is time interval in which the router advertises route updates
	AdvertiseEventsTick = 10 * time.Second
	// DefaultAdvertTTL is default advertisement TTL
	DefaultAdvertTTL = 2 * time.Minute
)

// router implements default router
type router struct {
	sync.RWMutex

	running   bool
	table     *table
	options   rr.Options
	exit      chan bool
	eventChan chan *rr.Event

	// advert subscribers
	sub         sync.RWMutex
	subscribers map[string]chan *rr.Advert
}

// newRouter creates new router and returns it
func newRouter(opts ...rr.Option) rr.Router {
	// get default options
	options := rr.DefaultOptions()

	// apply requested options
	for _, o := range opts {
		o(&options)
	}

	return &router{
		options:     options,
		table:       newTable(),
		subscribers: make(map[string]chan *rr.Advert),
	}
}

// Init initializes router with given options
func (r *router) Init(opts ...rr.Option) error {
	r.Lock()
	defer r.Unlock()

	for _, o := range opts {
		o(&r.options)
	}

	return nil
}

// Options returns router options
func (r *router) Options() rr.Options {
	r.RLock()
	defer r.RUnlock()

	options := r.options

	return options
}

// Table returns routing table
func (r *router) Table() rr.Table {
	return r.table
}

// manageRoute applies action on a given route
func (r *router) manageRoute(route rr.Route, action string) error {
	switch action {
	case "create":
		if err := r.table.Create(route); err != nil && err != ErrDuplicateRoute {
			return fmt.Errorf("failed adding route for service %s: %s", route.Service, err)
		}
	case "delete":
		if err := r.table.Delete(route); err != nil && err != ErrRouteNotFound {
			return fmt.Errorf("failed deleting route for service %s: %s", route.Service, err)
		}
	case "update":
		if err := r.table.Update(route); err != nil {
			return fmt.Errorf("failed updating route for service %s: %s", route.Service, err)
		}
	default:
		return fmt.Errorf("failed to manage route for service %s: unknown action %s", route.Service, action)
	}

	return nil
}

// manageServiceRoutes applies action to all routes of the service.
// It returns error of the action fails with error.
func (r *router) manageRoutes(service *regpb.Service, action string) error {
	// action is the routing table action
	action = strings.ToLower(action)

	// take route action on each service node
	for _, node := range service.Nodes {
		route := rr.Route{
			Service: service.Name,
			Address: node.Address,
			Gateway: "",
			Network: r.options.Network,
			Router:  r.options.Id,
			Link:    rr.DefaultLink,
			Metric:  rr.DefaultLocalMetric,
		}

		if err := r.manageRoute(route, action); err != nil {
			return err
		}
	}

	return nil
}

// manageRegistryRoutes applies action to all routes of each service found in the registry.
// It returns error if either the services failed to be listed or the routing table action fails.
func (r *router) manageRegistryRoutes(reg registry.Registry, action string) error {
	services, err := reg.ListServices()
	if err != nil {
		return fmt.Errorf("failed listing services: %v", err)
	}

	// add each service node as a separate route
	for _, service := range services {
		// get the service to retrieve all its info
		svcs, err := reg.GetService(service.Name)
		if err != nil {
			continue
		}
		// manage the routes for all returned services
		for _, svc := range svcs {
			if err := r.manageRoutes(svc, action); err != nil {
				return err
			}
		}
	}

	return nil
}

// watchRegistry watches registry and updates routing table based on the received events.
// It returns error if either the registry watcher fails with error or if the routing table update fails.
func (r *router) watchRegistry(w registry.Watcher) error {
	exit := make(chan bool)

	defer func() {
		close(exit)
	}()

	go func() {
		defer w.Stop()

		select {
		case <-exit:
			return
		case <-r.exit:
			return
		}
	}()

	for {
		res, err := w.Next()
		if err != nil {
			if err != registry.ErrWatcherStopped {
				return err
			}
			break
		}

		if err := r.manageRoutes(res.Service, res.Action); err != nil {
			return err
		}
	}

	return nil
}

// watchTable watches routing table entries and either adds or deletes locally registered service to/from network registry
// It returns error if the locally registered services either fails to be added/deleted to/from network registry.
func (r *router) watchTable(w rr.Watcher) error {
	exit := make(chan bool)

	defer func() {
		close(exit)
	}()

	// wait in the background for the router to stop
	// when the router stops, stop the watcher and exit
	go func() {
		defer w.Stop()

		select {
		case <-r.exit:
			return
		case <-exit:
			return
		}
	}()

	for {
		event, err := w.Next()
		if err != nil {
			if err != rr.ErrWatcherStopped {
				return err
			}
			break
		}

		select {
		case <-r.exit:
			close(r.eventChan)
			return nil
		case r.eventChan <- event:
			// process event
		}
	}

	return nil
}

// publishAdvert publishes router advert to advert channel
func (r *router) publishAdvert(advType rr.AdvertType, events []*rr.Event) {
	a := &rr.Advert{
		Id:        r.options.Id,
		Type:      advType,
		TTL:       DefaultAdvertTTL,
		Timestamp: time.Now(),
		Events:    events,
	}

	r.sub.RLock()
	for _, sub := range r.subscribers {
		// now send the message
		select {
		case sub <- a:
		case <-r.exit:
			r.sub.RUnlock()
			return
		}
	}
	r.sub.RUnlock()
}

// adverts maintains a map of router adverts
type adverts map[uint64]*rr.Event

// advertiseEvents advertises routing table events
// It suppresses unhealthy flapping events and advertises healthy events upstream.
func (r *router) advertiseEvents() error {
	// ticker to periodically scan event for advertising
	ticker := time.NewTicker(AdvertiseEventsTick)
	defer ticker.Stop()

	// adverts is a map of advert events
	adverts := make(adverts)

	// routing table watcher
	w, err := r.Watch()
	if err != nil {
		return err
	}
	defer w.Stop()

	go func() {
		var err error

		for {
			select {
			case <-r.exit:
				return
			default:
				if w == nil {
					// routing table watcher
					w, err = r.Watch()
					if err != nil {
						log.Errorf("Error creating watcher: %v", err)
						time.Sleep(time.Second)
						continue
					}
				}

				if err := r.watchTable(w); err != nil {
					log.Errorf("Error watching table: %v", err)
					time.Sleep(time.Second)
				}

				if w != nil {
					// reset
					w.Stop()
					w = nil
				}
			}
		}
	}()

	for {
		select {
		case <-ticker.C:
			// If we're not advertising any events then sip processing them entirely
			if r.options.Advertise == rr.AdvertiseNone {
				continue
			}

			var events []*rr.Event

			// collect all events which are not flapping
			for key, event := range adverts {
				// if we only advertise local routes skip processing anything not link local
				if r.options.Advertise == rr.AdvertiseLocal && event.Route.Link != "local" {
					continue
				}

				// copy the event and append
				e := new(rr.Event)
				// this is ok, because router.Event only contains builtin types
				// and no references so this creates a deep copy of struct Event
				*e = *event
				events = append(events, e)
				// delete the advert from adverts
				delete(adverts, key)
			}

			// advertise events to subscribers
			if len(events) > 0 {
				log.Debugf("Router publishing %d events", len(events))
				go r.publishAdvert(rr.RouteUpdate, events)
			}
		case e := <-r.eventChan:
			// if event is nil, continue
			if e == nil {
				continue
			}

			// If we're not advertising any events then skip processing them entirely
			if r.options.Advertise == rr.AdvertiseNone {
				continue
			}

			// if we only advertise local routes skip processing anything not link local
			if r.options.Advertise == rr.AdvertiseLocal && e.Route.Link != "local" {
				continue
			}

			log.Debugf("Router processing table event %s for service %s %s", e.Type, e.Route.Service, e.Route.Address)

			// check if we have already registered the route
			hash := e.Route.Hash()
			ev, ok := adverts[hash]
			if !ok {
				ev = e
				adverts[hash] = e
				continue
			}

			// override the route event only if the previous event was different
			if ev.Type != e.Type {
				ev = e
			}
		case <-r.exit:
			if w != nil {
				w.Stop()
			}
			return nil
		}
	}
}

// drain all the events, only called on Stop
func (r *router) drain() {
	for {
		select {
		case <-r.eventChan:
		default:
			return
		}
	}
}

// Start starts the router
func (r *router) Start() error {
	r.Lock()
	defer r.Unlock()

	if r.running {
		return nil
	}

	// add all local service routes into the routing table
	if err := r.manageRegistryRoutes(r.options.Registry, "create"); err != nil {
		return fmt.Errorf("failed adding registry routes: %s", err)
	}

	// add default gateway into routing table
	if r.options.Gateway != "" {
		// note, the only non-default value is the gateway
		route := rr.Route{
			Service: "*",
			Address: "*",
			Gateway: r.options.Gateway,
			Network: "*",
			Router:  r.options.Id,
			Link:    rr.DefaultLink,
			Metric:  rr.DefaultLocalMetric,
		}
		if err := r.table.Create(route); err != nil {
			return fmt.Errorf("failed adding default gateway route: %s", err)
		}
	}

	// create error and exit channels
	r.exit = make(chan bool)

	// registry watcher
	w, err := r.options.Registry.Watch()
	if err != nil {
		return fmt.Errorf("failed creating registry watcher: %v", err)
	}

	go func() {
		var err error

		for {
			select {
			case <-r.exit:
				if w != nil {
					w.Stop()
				}
				return
			default:
				if w == nil {
					w, err = r.options.Registry.Watch()
					if err != nil {
						log.Errorf("failed creating registry watcher: %v", err)
						time.Sleep(time.Second)
						continue
					}
				}

				if err := r.watchRegistry(w); err != nil {
					log.Errorf("Error watching the registry: %v", err)
					time.Sleep(time.Second)
				}

				if w != nil {
					w.Stop()
					w = nil
				}
			}
		}
	}()

	r.running = true

	return nil
}

// Advertise stars advertising the routes to the network and returns the advertisements channel to consume from.
// If the router is already advertising it returns the channel to consume from.
// It returns error if either the router is not running or if the routing table fails to list the routes to advertise.
func (r *router) Advertise() (<-chan *rr.Advert, error) {
	r.Lock()
	defer r.Unlock()

	if !r.running {
		return nil, errors.New("not running")
	}

	// already advertising
	if r.eventChan != nil {
		advertChan := make(chan *rr.Advert, 128)
		r.subscribers[uuid.New().String()] = advertChan
		return advertChan, nil
	}

	// list all the routes and pack them into even slice to advertise
	events, err := r.flushRouteEvents(rr.Create)
	if err != nil {
		return nil, fmt.Errorf("failed to flush routes: %s", err)
	}

	// create event channels
	r.eventChan = make(chan *rr.Event)

	// create advert channel
	advertChan := make(chan *rr.Advert, 128)
	r.subscribers[uuid.New().String()] = advertChan

	// advertise your presence
	go r.publishAdvert(rr.Announce, events)

	go func() {
		select {
		case <-r.exit:
			return
		default:
			if err := r.advertiseEvents(); err != nil {
				log.Errorf("Error adveritising events: %v", err)
			}
		}
	}()

	return advertChan, nil

}

// Process updates the routing table using the advertised values
func (r *router) Process(a *rr.Advert) error {
	// NOTE: event sorting might not be necessary
	// copy update events intp new slices
	events := make([]*rr.Event, len(a.Events))
	copy(events, a.Events)
	// sort events by timestamp
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	log.Debugf("Router %s processing advert from: %s", r.options.Id, a.Id)

	for _, event := range events {
		// skip if the router is the origin of this route
		if event.Route.Router == r.options.Id {
			log.Debugf("Router skipping processing its own route: %s", r.options.Id)
			continue
		}
		// create a copy of the route
		route := event.Route
		action := event.Type

		log.Debugf("Router %s applying %s from router %s for service %s %s", r.options.Id, action, route.Router, route.Service, route.Address)

		if err := r.manageRoute(route, action.String()); err != nil {
			return fmt.Errorf("failed applying action %s to routing table: %s", action, err)
		}
	}

	return nil
}

// flushRouteEvents returns a slice of events, one per each route in the routing table
func (r *router) flushRouteEvents(evType rr.EventType) ([]*rr.Event, error) {
	// get a list of routes for each service in our routing table
	// for the configured advertising strategy
	q := []rr.QueryOption{
		rr.QueryStrategy(r.options.Advertise),
	}

	routes, err := r.Table().Query(q...)
	if err != nil && err != ErrRouteNotFound {
		return nil, err
	}

	log.Debugf("Router advertising %d routes with strategy %s", len(routes), r.options.Advertise)

	// build a list of events to advertise
	events := make([]*rr.Event, len(routes))
	var i int

	for _, route := range routes {
		event := &rr.Event{
			Type:      evType,
			Timestamp: time.Now(),
			Route:     route,
		}
		events[i] = event
		i++
	}

	return events, nil
}

// Lookup routes in the routing table
func (r *router) Lookup(q ...rr.QueryOption) ([]rr.Route, error) {
	return r.table.Query(q...)
}

// Watch routes
func (r *router) Watch(opts ...rr.WatchOption) (rr.Watcher, error) {
	return r.table.Watch(opts...)
}

// Stop stops the router
func (r *router) Stop() error {
	r.Lock()
	defer r.Unlock()

	select {
	case <-r.exit:
		return nil
	default:
		close(r.exit)

		// extract the events
		r.drain()

		r.sub.Lock()
		// close advert subscribers
		for id, sub := range r.subscribers {
			// close the channel
			close(sub)
			// delete the subscriber
			delete(r.subscribers, id)
		}
		r.sub.Unlock()
	}

	// remove event chan
	r.eventChan = nil

	return nil
}

// String prints debugging information about router
func (r *router) String() string {
	return "registry"
}

// NewRouter creates new Router and returns it
func NewRouter(opts ...rr.Option) rr.Router {
	return newRouter(opts...)
}

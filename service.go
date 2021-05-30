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

package vine

import (
	"os"
	"os/signal"
	"runtime"
	"sync"

	"github.com/lack-io/vine/core/client"
	"github.com/lack-io/vine/core/server"
	"github.com/lack-io/vine/lib/cmd"
	"github.com/lack-io/vine/lib/debug/handler"
	"github.com/lack-io/vine/lib/debug/stats"
	"github.com/lack-io/vine/lib/debug/trace"
	"github.com/lack-io/vine/lib/logger"
	"github.com/lack-io/vine/lib/store"
	signalutil "github.com/lack-io/vine/util/signal"
	"github.com/lack-io/vine/util/wrapper"
)

type service struct {
	opts Options

	once sync.Once
}

func newService(opts ...Option) Service {
	sv := new(service)
	options := newOptions(opts...)

	// service name
	serviceName := options.Server.Options().Name

	// wrap client to inject From-Service header on any calls
	options.Client = wrapper.FromService(serviceName, options.Client)
	options.Client = wrapper.TraceCall(serviceName, trace.DefaultTracer, options.Client)

	// wrap the server to provided handler stats
	_ = options.Server.Init(
		server.WrapHandler(wrapper.HandlerStats(stats.DefaultStats)),
		server.WrapHandler(wrapper.TraceHandler(trace.DefaultTracer)),
	)

	// set opts
	sv.opts = options

	return sv
}

func (s *service) Name() string {
	return s.opts.Server.Options().Name
}

// Init initialises options. Additionally it calls cmd.Init
// which parses command line flags. cmd.Init is only called
// on first Init.
func (s *service) Init(opts ...Option) {
	// process options
	for _, o := range opts {
		o(&s.opts)
	}

	s.once.Do(func() {
		if s.opts.Cmd != nil {
			// set cmd name
			if len(s.opts.Cmd.App().Name) == 0 {
				s.opts.Cmd.App().Name = s.Server().Options().Name
			}

			// Initialise the command flags, overriding new service
			if err := s.opts.Cmd.Init(
				cmd.Broker(&s.opts.Broker),
				cmd.Registry(&s.opts.Registry),
				cmd.Runtime(&s.opts.Runtime),
				cmd.Transport(&s.opts.Transport),
				cmd.Client(&s.opts.Client),
				cmd.Config(&s.opts.Config),
				cmd.Server(&s.opts.Server),
				cmd.Store(&s.opts.Store),
				cmd.Dialect(&s.opts.Dialect),
				cmd.Profile(&s.opts.Profile),
			); err != nil {
				logger.Fatal(err)
			}
		}

		s.opts.BeforeStop = append(s.opts.BeforeStop, func() error {
			s.opts.Scheduler.Stop()
			return nil
		})

		// Explicitly set the table name to the service name
		name := s.opts.Server.Options().Name
		_ = s.opts.Store.Init(store.Table(name))
	})
}

func (s *service) Options() Options {
	return s.opts
}

func (s *service) Client() client.Client {
	return s.opts.Client
}

func (s *service) Server() server.Server {
	return s.opts.Server
}

func (s *service) Start() error {
	for _, fn := range s.opts.BeforeStart {
		if err := fn(); err != nil {
			return err
		}
	}

	if err := s.opts.Server.Start(); err != nil {
		return err
	}

	for _, fn := range s.opts.AfterStart {
		if err := fn(); err != nil {
			return err
		}
	}

	return nil
}

func (s *service) Stop() error {
	var gerr error

	for _, fn := range s.opts.BeforeStop {
		if err := fn(); err != nil {
			gerr = err
		}
	}

	if err := s.opts.Server.Stop(); err != nil {
		return err
	}

	for _, fn := range s.opts.AfterStop {
		if err := fn(); err != nil {
			gerr = err
		}
	}

	return gerr
}

func (s *service) Run() error {
	// register the debug handler
	if err := s.opts.Server.Handle(
		s.opts.Server.NewHandler(
			handler.NewHandler(s.opts.Client),
			server.InternalHandler(true),
		),
	); err != nil {
		return err
	}

	// start the profiler
	if s.opts.Profile != nil {
		// to view mutex contention
		runtime.SetMutexProfileFraction(5)
		// to view blocking profile
		runtime.SetBlockProfileRate(1)

		if err := s.opts.Profile.Start(); err != nil {
			return err
		}
		defer s.opts.Profile.Stop()
	}

	// start the profiler
	logger.Infof("Starting [service] %s", s.Name())
	logger.Infof("service [version] %s", s.Options().Server.Options().Version)

	if err := s.Start(); err != nil {
		return err
	}

	ch := make(chan os.Signal, 1)
	if s.opts.Signal {
		signal.Notify(ch, signalutil.Shutdown()...)
	}

	select {
	// wait on kill signal
	case <-ch:
	// wait on context cancel
	case <-s.opts.Context.Done():
	}

	return s.Stop()
}

func (s *service) String() string {
	return "vine"
}

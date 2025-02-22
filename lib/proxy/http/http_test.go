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

package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"

	"github.com/lack-io/vine"
	"github.com/lack-io/vine/core/client"
	"github.com/lack-io/vine/core/server"
)

type testHandler struct{}

func (t *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"hello": "world"}`))
}

func TestHTTPRouter(t *testing.T) {
	t.Log("skip broken test")
	t.Skip()
	c, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	addr := c.Addr().String()

	url := fmt.Sprintf("http://%s", addr)

	testCases := []struct {
		// local url e.g http://localhost:9090
		url string
		// http endpoint to call e.g /foo/bar
		httpEp string
		// rpc endpoint called e.g Foo.Bar
		rpcEp string
		// should be an error
		err bool
	}{
		{addr, "/foo/bar", "Foo.Bar", false},
		{addr, "/foo/baz", "Foo.Baz", true},
		{addr, "/helloworld", "Hello.World", false},
		{addr, "/greeter", "Greeter.Hello", false},
		{addr, "/", "Fail.Hard", true},
	}

	// handler
	http.Handle("/foo/bar", new(testHandler))
	http.Handle("/helloworld", new(testHandler))
	http.Handle("/greeter", new(testHandler))

	// new proxy
	p := NewSingleHostRouter(url)

	// register a route
	p.RegisterEndpoint("Hello.World", "/helloworld")
	p.RegisterEndpoint("Greeter.Hello", url+"/greeter")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	// new vine service
	service := vine.NewService(
		vine.Context(ctx),
		vine.Name("foobar"),
		//vine.Registry(memory.NewRegistry()),
		vine.AfterStart(func() error {
			wg.Done()
			return nil
		}),
	)

	// set router
	service.Server().Init(
		server.WithRouter(p),
	)

	// run service
	// server
	go http.Serve(c, nil)
	go service.Run()

	// wait till service is started
	wg.Wait()

	for _, test := range testCases {
		req := service.Client().NewRequest("foobar", test.rpcEp, map[string]string{"foo": "bar"}, client.WithContentType("application/json"))
		var rsp map[string]string
		err := service.Client().Call(ctx, req, &rsp)
		if err != nil && test.err == false {
			t.Fatal(err)
		}
		if err == nil && test.err == true {
			t.Fatalf("Expected error for %v:%v got %v and response %v", test.rpcEp, test.httpEp, err, rsp)
		} else {
			continue
		}
		if v := rsp["hello"]; v != "world" {
			t.Fatalf("Expected hello world got %s from %s", v, test.rpcEp)
		}
	}
}

func TestHTTPRouterOptions(t *testing.T) {
	// test endpoint
	service := NewService(
		WithBackend("http://foo.bar"),
	)

	r := service.Server().Options().Router
	httpRouter, ok := r.(*Router)
	if !ok {
		t.Fatal("Expected http router to be installed")
	}
	if httpRouter.Backend != "http://foo.bar" {
		t.Fatalf("Expected endpoint http://foo.bar got %v", httpRouter.Backend)
	}

	// test router
	service = NewService(
		WithRouter(&Router{Backend: "http://foo2.bar"}),
	)
	r = service.Server().Options().Router
	httpRouter, ok = r.(*Router)
	if !ok {
		t.Fatal("Expected http router to be installed")
	}
	if httpRouter.Backend != "http://foo2.bar" {
		t.Fatalf("Expected endpoint http://foo2.bar got %v", httpRouter.Backend)
	}
}

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

package transport

import (
	"sync"
	"testing"
)

func call(b *testing.B, c int) {
	b.StopTimer()

	tr := NewTransport()

	// server listen
	l, err := tr.Listen("localhost:0")
	if err != nil {
		b.Fatal(err)
	}
	defer l.Close()

	// socket func
	fn := func(sock Socket) {
		defer sock.Close()

		for {
			var m Message
			if err := sock.Recv(&m); err != nil {
				return
			}

			if err := sock.Send(&m); err != nil {
				return
			}
		}
	}

	done := make(chan bool)

	// accept connections
	go func() {
		if err := l.Accept(fn); err != nil {
			select {
			case <-done:
			default:
				b.Fatalf("Unexpected accept err: %v", err)
			}
		}
	}()

	m := Message{
		Header: map[string]string{
			"Content-Type": "application/json",
		},
		Body: []byte(`{"message": "Hello World"}`),
	}

	// client connection
	client, err := tr.Dial(l.Addr())
	if err != nil {
		b.Fatalf("Unexpected dial err: %v", err)
	}

	send := func(c Client) {
		// send message
		if err := c.Send(&m); err != nil {
			b.Fatalf("Unexpected send err: %v", err)
		}

		var rm Message
		// receive message
		if err := c.Recv(&rm); err != nil {
			b.Fatalf("Unexpected recv err: %v", err)
		}
	}

	// warm
	for i := 0; i < 10; i++ {
		send(client)
	}

	client.Close()

	ch := make(chan int, c*4)

	var wg sync.WaitGroup
	wg.Add(c)

	for i := 0; i < c; i++ {
		go func() {
			cl, err := tr.Dial(l.Addr())
			if err != nil {
				b.Fatalf("Unexpected dial err: %v", err)
			}
			defer cl.Close()

			for range ch {
				send(cl)
			}

			wg.Done()
		}()
	}

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		ch <- i
	}

	b.StopTimer()
	close(ch)

	wg.Wait()

	// finish
	close(done)
}

func BenchmarkTransport1(b *testing.B) {
	call(b, 1)
}

func BenchmarkTransport8(b *testing.B) {
	call(b, 8)
}

func BenchmarkTransport16(b *testing.B) {
	call(b, 16)
}

func BenchmarkTransport64(b *testing.B) {
	call(b, 64)
}

func BenchmarkTransport128(b *testing.B) {
	call(b, 128)
}

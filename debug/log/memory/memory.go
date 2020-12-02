// Copyright 2020 The vine Authors
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

// Package memory provides an in memory log buffer
package memory

import (
	"fmt"

	"github.com/lack-io/vine/debug/log"
	"github.com/lack-io/vine/util/ring"
)

var (
	// DefaultSize of the logger buffer
	DefaultSize = 1024
)

// memoryLog is default micro log
type memoryLog struct {
	*ring.Buffer
}

// NewLog returns default Logger with
func NewLog(opts ...log.Option) log.Log {
	// get default options
	options := log.DefaultOptions()

	// apply requested options
	for _, o := range opts {
		o(&options)
	}

	return &memoryLog{
		Buffer: ring.New(options.Size),
	}
}

// Write writes logs into logger
func (l *memoryLog) Write(r log.Record) error {
	l.Buffer.Put(fmt.Sprint(r.Message))
	return nil
}

// Read reads logs and returns them
func (l *memoryLog) Read(opts ...log.ReadOption) ([]log.Record, error) {
	options := log.ReadOptions{}
	// initialize the read options
	for _, o := range opts {
		o(&options)
	}

	var entries []*ring.Entry
	// if Since options ha sbeen specified we honor it
	if !options.Since.IsZero() {
		entries = l.Buffer.Since(options.Since)
	}

	// only if we specified valid count constraint
	// do we end up doing some serious if-else kung-fu
	// if since constraint has been provided
	// we return *count* number of logs since the given timestamp;
	// otherwise we return last count number of logs
	if options.Count > 0 {
		switch len(entries) > 0 {
		case true:
			// if we request fewer logs than what since constraint gives us
			if options.Count < len(entries) {
				entries = entries[0:options.Count]
			}
		default:
			entries = l.Buffer.Get(options.Count)
		}
	}

	records := make([]log.Record, 0, len(entries))
	for _, entry := range entries {
		record := log.Record{
			Timestamp: entry.Timestamp,
			Message:   entry.Value,
		}
		records = append(records, record)
	}

	return records, nil
}

// Stream returns channel for reading log records
// along with a stop channel, close it when done
func (l *memoryLog) Stream() (log.Stream, error) {
	// get stream channel from ring buffer
	stream, stop := l.Buffer.Stream()
	// make a buffered channel
	records := make(chan log.Record, 128)
	// get last 10 records
	last10 := l.Buffer.Get(10)

	// stream the log records
	go func() {
		// first send last 10 records
		for _, entry := range last10 {
			records <- log.Record{
				Timestamp: entry.Timestamp,
				Message:   entry.Value,
				Metadata:  make(map[string]string),
			}
		}
		// now stream continuously
		for entry := range stream {
			records <- log.Record{
				Timestamp: entry.Timestamp,
				Message:   entry.Value,
				Metadata:  make(map[string]string),
			}
		}
	}()

	return &logStream{
		stream: records,
		stop:   stop,
	}, nil
}

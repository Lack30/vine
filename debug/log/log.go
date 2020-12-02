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

// Package log provides debug logging
package log

import (
	"fmt"
	"time"

	json "github.com/json-iterator/go"
)

var (
	// Default buffer size if any
	DefaultSize = 1024
	// DefaultLog logger
	DefaultLog = NewLog()
	// Default formatter
	DefaultFormat = TextFormat
)

// Log is debug log interface for reading and writing logs
type Log interface {
	// Read reads log entries from the logger
	Read(...ReadOption) ([]Record, error)
	// Write writes records to log
	Write(Record) error
	// Stream log records
	Stream() (Stream, error)
}

// Record is log record entry
type Record struct {
	// Timestamp of logged event
	Timestamp time.Time `json:"timestamp"`
	// Metadata to enrich log record
	Metadata map[string]string `json:"metadata"`
	// Value contains log entry
	Message interface{} `json:"message"`
}

// Stream returns a log stream
type Stream interface {
	Chan() <-chan Record
	Stop() error
}

// Format is a function which formats the output
type FormatFunc func(Record) string

// TextFormat returns text format
func TextFormat(r Record) string {
	t := r.Timestamp.Format("2006/01/02 15:04:05")
	return fmt.Sprintf("%s %v", t, r.Message)
}

// JSONFormat is a json Format func
func JSONFormat(r Record) string {
	b, _ := json.Marshal(r)
	return string(b)
}

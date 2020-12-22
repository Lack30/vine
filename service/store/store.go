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

// Package store is an interface for distributed data storage.
package store

import (
	"errors"
	"time"
)

var (
	// DefaultStore implementation
	DefaultStore Store
	// DefaultBlobStore implementation
	DefaultBlobStore BlobStore
	// ErrNotFound is returned when a key doesn't exist
	ErrNotFound = errors.New("not found")
)

// Store is a data storage interface
type Store interface {
	// Init initialises the store. It must perform any required setup on the backing storage implementation and check that it is ready for use, returning any errors.
	Init(...StoreOption) error
	// Options allows you to view the current options.
	Options() StoreOptions
	// Read takes a single key name and optional ReadOptions. It returns matching []*Record or an error.
	Read(key string, opts ...ReadOption) ([]*Record, error)
	// Write() writes a record to the store, and returns an error if the record was not written.
	Write(r *Record, opts ...WriteOption) error
	// Delete removes the record with the corresponding key from the store.
	Delete(key string, opts ...DeleteOption) error
	// List returns any keys that match, or an empty list with no error if none matched.
	List(opts ...ListOption) ([]string, error)
	// Close the store
	Close() error
	// String returns the name of the implementation.
	String() string
}

// Record is an item stored or retrieved from a Store
type Record struct {
	// The key to store the record
	Key string `json:"key"`
	// The value within the record
	Value []byte `json:"value"`
	// Any associated metadata for indexing
	Metadata map[string]interface{} `json:"metadata"`
	// Time to expire a record: TODO: change to timestamp
	Expiry time.Duration `json:"expiry,omitempty"`
}

// Read records
func Read(key string, opts ...Option) ([]*Record, error) {
	var options Options
	for _, o := range opts {
		o(&options)
	}

	// convert the options
	var readOpts []ReadOption
	if len(options.Prefix) > 0 {
		key = options.Prefix
		readOpts = append(readOpts, ReadPrefix())
	}
	if options.Limit > 0 {
		readOpts = append(readOpts, ReadLimit(options.Limit))
	}
	if options.Offset > 0 {
		readOpts = append(readOpts, ReadOffset(options.Offset))
	}

	// execute the query
	return DefaultStore.Read(key, readOpts...)
}

// Write a record to the store
func Write(r *Record) error {
	return DefaultStore.Write(r)
}

// Delete removes the record with the corresponding key from the store.
func Delete(key string) error {
	return DefaultStore.Delete(key)
}

// List returns any keys that match, or an empty list with no error if none matched.
func List(opts ...Option) ([]string, error) {
	var options Options
	for _, o := range opts {
		o(&options)
	}

	// convert the options
	var listOpts []ListOption
	if len(options.Prefix) > 0 {
		listOpts = append(listOpts, ListPrefix(options.Prefix))
	}
	if options.Limit > 0 {
		listOpts = append(listOpts, ListLimit(options.Limit))
	}
	if options.Offset > 0 {
		listOpts = append(listOpts, ListOffset(options.Offset))
	}

	return DefaultStore.List(listOpts...)
}

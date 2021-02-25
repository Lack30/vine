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

// Package cache implements a faulting style read cache on top of multiple vine stores
package cache

import (
	"fmt"

	"github.com/lack-io/vine/service/store"
	"github.com/lack-io/vine/service/store/memory"
)

type cache struct {
	stores []store.Store
}

// Cache is a cpu register style cache for the store.
// It syncs between N stores in a faulting manner.
type Cache interface {
	// Implements the store interface
	store.Store
}

// NewCache returns a new store using the underlying stores, which must be already Init()ialised
func NewCache(stores ...store.Store) Cache {
	if len(stores) == 0 {
		stores = []store.Store{
			memory.NewStore(),
		}
	}

	// TODO: build in an in memory cache
	c := &cache{
		stores: stores,
	}

	return c
}

func (c *cache) Close() error {
	return nil
}

func (c *cache) Init(opts ...store.Option) error {
	// pass to the stores
	for _, store := range c.stores {
		if err := store.Init(opts...); err != nil {
			return err
		}
	}
	return nil
}

func (c *cache) Options() store.Options {
	// return from first store
	return c.stores[0].Options()
}

func (c *cache) String() string {
	stores := make([]string, len(c.stores))
	for i, s := range c.stores {
		stores[i] = s.String()
	}
	return fmt.Sprintf("cache %v", stores)
}

func (c *cache) Read(key string, opts ...store.ReadOption) ([]*store.Record, error) {
	readOpts := store.ReadOptions{}
	for _, o := range opts {
		o(&readOpts)
	}

	if readOpts.Prefix || readOpts.Suffix {
		// List, then try cached gets for each key
		var lOpts []store.ListOption
		if readOpts.Prefix {
			lOpts = append(lOpts, store.ListPrefix(key))
		}
		if readOpts.Suffix {
			lOpts = append(lOpts, store.ListSuffix(key))
		}
		if readOpts.Limit > 0 {
			lOpts = append(lOpts, store.ListLimit(readOpts.Limit))
		}
		if readOpts.Offset > 0 {
			lOpts = append(lOpts, store.ListOffset(readOpts.Offset))
		}
		keys, err := c.List(lOpts...)
		if err != nil {
			return []*store.Record{}, fmt.Errorf("%w: cache.List failed", err)
		}
		recs := make([]*store.Record, len(keys))
		for i, k := range keys {
			r, err := c.readOne(k, opts...)
			if err != nil {
				return recs, fmt.Errorf("%w: cache.readOne failed", err)
			}
			recs[i] = r
		}
		return recs, nil
	}

	// Otherwise just try cached get
	r, err := c.readOne(key, opts...)
	if err != nil {
		return []*store.Record{}, err // preserve store.ErrNotFound
	}
	return []*store.Record{r}, nil
}

func (c *cache) readOne(key string, opts ...store.ReadOption) (*store.Record, error) {
	for i, s := range c.stores {
		// ReadOne ignores all options
		r, err := s.Read(key)
		if err == nil {
			if len(r) > 1 {
				return nil, fmt.Errorf("%w: read from L%d cache (%s) returned multiple records", err, i, c.stores[i].String())
			}
			for j := i - 1; j >= 0; j-- {
				err := c.stores[j].Write(r[0])
				if err != nil {
					return nil, fmt.Errorf("%w: could not write to L%d cache (%s)", err, j, c.stores[j].String())
				}
			}
			return r[0], nil
		}
	}
	return nil, store.ErrNotFound
}

func (c *cache) Write(r *store.Record, opts ...store.WriteOption) error {
	// Write to all layers in reverse
	for i := len(c.stores) - 1; i >= 0; i-- {
		if err := c.stores[i].Write(r, opts...); err != nil {
			return fmt.Errorf("%w: could not write to L%d cache (%s)", err, i, c.stores[i].String())
		}
	}
	return nil
}

func (c *cache) Delete(key string, opts ...store.DeleteOption) error {
	for i, s := range c.stores {
		if err := s.Delete(key, opts...); err != nil {
			return fmt.Errorf("%w: could not delete from L%d cache (%s)", err, i, c.stores[i].String())
		}
	}
	return nil
}

func (c *cache) List(opts ...store.ListOption) ([]string, error) {
	// List only makes sense from the top level
	return c.stores[len(c.stores)-1].List(opts...)
}

// MIT License
//
// Copyright (c) 2021 Lack
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

package runtime

import (
	"context"
	"strings"
	"sync"

	"github.com/lack-io/vine/lib/dao"
	"github.com/lack-io/vine/lib/dao/clause"
)

// GroupVersionKind contains the information of Object, etc Group, Version, Kind
type GroupVersionKind struct {
	Group   string
	Version string
	Kind    string
}

func (gvk *GroupVersionKind) APIGroup() string {
	if gvk.Group == "" {
		return gvk.Version
	}
	return gvk.Group + "/" + gvk.Version
}

func (gvk *GroupVersionKind) String() string {
	var s string
	if gvk.Group != "" {
		s = gvk.Group + "/"
	}
	if gvk.Version != "" {
		s = s + gvk.Version + "."
	}
	return s + gvk.Kind
}

func FromGVK(s string) *GroupVersionKind {
	gvk := &GroupVersionKind{}
	if idx := strings.Index(s, "/"); idx != -1 {
		gvk.Group = s[:idx]
		s = s[idx+1:]
	}
	if idx := strings.Index(s, "."); idx != -1 {
		gvk.Version = s[:idx]
		s = s[idx+1:]
	} else {
		gvk.Version = "v1"
	}
	gvk.Kind = s
	return gvk
}

// Object is an interface that describes protocol message
type Object interface {
	// GVK get the GroupVersionKind of Object
	GVK() *GroupVersionKind
	// DeepCopyFrom deep copy the struct from another
	DeepCopyFrom(Object)
	// DeepCopy deep copy the struct
	DeepCopy() Object
}

var oset = NewObjectSet()

type ObjectSet struct {
	sync.RWMutex

	sets map[string]Object

	OnCreate func(in Object) Object
}

// NewObj creates a new object, trigger OnCreate function
func (os *ObjectSet) NewObj(gvk string) (Object, bool) {
	os.RLock()
	defer os.RUnlock()
	out, ok := os.sets[gvk]
	if !ok {
		return nil, false
	}
	return os.OnCreate(out.DeepCopy()), true
}

// NewObjWithGVK creates a new object, trigger OnCreate function
func (os *ObjectSet) NewObjWithGVK(gvk *GroupVersionKind) (Object, bool) {
	os.RLock()
	defer os.RUnlock()
	out, ok := os.sets[gvk.String()]
	if !ok {
		return nil, false
	}
	return os.OnCreate(out.DeepCopy()), true
}

func (os *ObjectSet) IsExists(gvk *GroupVersionKind) bool {
	os.RLock()
	defer os.RUnlock()
	_, ok := os.sets[gvk.String()]
	return ok
}

func (os *ObjectSet) GetObj(gvk *GroupVersionKind) (Object, bool) {
	os.RLock()
	defer os.RUnlock()
	out, ok := os.sets[gvk.String()]
	if !ok {
		return nil, false
	}
	return out.DeepCopy(), true
}

// AddObj push objects to Set
func (os *ObjectSet) AddObj(v ...Object) {
	os.Lock()
	for _, in := range v {
		os.sets[in.GVK().String()] = in
	}
	os.Unlock()
}

// NewObj creates a new object, trigger OnCreate function
func NewObj(gvk string) (Object, bool) {
	return oset.NewObj(gvk)
}

// NewObjWithGVK creates a new object, trigger OnCreate function
func NewObjWithGVK(gvk *GroupVersionKind) (Object, bool) {
	return oset.NewObjWithGVK(gvk)
}

func AddObj(v ...Object) {
	oset.AddObj(v...)
}

func NewObjectSet() *ObjectSet {
	return &ObjectSet{
		sets:     map[string]Object{},
		OnCreate: func(in Object) Object { return in },
	}
}

type Schema interface {
	FindPage(ctx context.Context, page, size int32) ([]Object, int64, error)
	FindAll(ctx context.Context) ([]Object, error)
	FindPureAll(ctx context.Context) ([]Object, error)
	Count(ctx context.Context) (total int64, err error)
	FindOne(ctx context.Context) (Object, error)
	FindPureOne(ctx context.Context) (Object, error)
	Cond(exprs ...clause.Expression) Schema
	Create(ctx context.Context) (Object, error)
	BatchUpdates(ctx context.Context) error
	Updates(ctx context.Context) (Object, error)
	BatchDelete(ctx context.Context, soft bool) error
	Delete(ctx context.Context, soft bool) error
	Tx(ctx context.Context) *dao.DB
}

var sset = SchemaSet{sets: map[string]func(Object) Schema{}}

type SchemaSet struct {
	sync.RWMutex
	sets map[string]func(Object) Schema
}

func (s *SchemaSet) RegistrySchema(g *GroupVersionKind, fn func(Object) Schema) {
	s.Lock()
	s.sets[g.String()] = fn
	s.Unlock()
}

func (s *SchemaSet) NewSchema(in Object) (Schema, bool) {
	s.RLock()
	defer s.RUnlock()
	fn, ok := s.sets[in.GVK().String()]
	if !ok {
		return nil, false
	}
	return fn(in), true
}

func NewSchemaSet() *SchemaSet {
	return &SchemaSet{sets: map[string]func(Object) Schema{}}
}

func RegistrySchema(g *GroupVersionKind, fn func(Object) Schema) {
	sset.RegistrySchema(g, fn)
}

func NewSchema(in Object) (Schema, bool) {
	return sset.NewSchema(in)
}

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

package auth

import (
	"github.com/google/uuid"

	"github.com/lack-io/vine/service/auth/provider/basic"
)

var (
	DefaultAuth = NewAuth()
)

func NewAuth(opts ...Option) Auth {
	options := Options{
		Provider: basic.NewProvider(),
	}

	for _, o := range opts {
		o(&options)
	}

	return &noop{
		opts: options,
	}
}

type noop struct {
	opts Options
}

// String returns the name of the implementation
func (n *noop) String() string {
	return "noop"
}

// Init the auth
func (n *noop) Init(opts ...Option) {
	for _, o := range opts {
		o(&n.opts)
	}
}

// Options set for auth
func (n *noop) Options() Options {
	return n.opts
}

// Generate a new account
func (n *noop) Generate(id string, opts ...GenerateOption) (*Account, error) {
	options := NewGenerateOptions(opts...)

	return &Account{
		ID:       id,
		Secret:   options.Secret,
		Metadata: options.Metadata,
		Scopes:   options.Scopes,
		Issuer:   n.Options().Namespace,
	}, nil
}

// Grant access to a resource
func (n *noop) Grant(rule *Rule) error {
	return nil
}

// Revoke access to a resource
func (n *noop) Revoke(rule *Rule) error {
	return nil
}

// Rules used to verify requests
func (n *noop) Rules(opts ...RulesOption) ([]*Rule, error) {
	return []*Rule{}, nil
}

// Verify an account has access to a resource
func (n *noop) Verify(acc *Account, res *Resource, opts ...VerifyOption) error {
	return nil
}

// Inspect a token
func (n *noop) Inspect(token string) (*Account, error) {
	return &Account{ID: uuid.New().String(), Issuer: n.Options().Namespace}, nil
}

// Token generation using an account id and secret
func (n *noop) Token(opts ...TokenOption) (*Token, error) {
	return &Token{}, nil
}

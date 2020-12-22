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

package client

import (
	"strings"
	"time"

	"github.com/lack-io/vine/internal/auth/rules"
	"github.com/lack-io/vine/internal/auth/token"
	"github.com/lack-io/vine/internal/auth/token/jwt"
	pb "github.com/lack-io/vine/proto/auth"
	"github.com/lack-io/vine/service/auth"
	"github.com/lack-io/vine/service/client"
	"github.com/lack-io/vine/service/client/cache"
	"github.com/lack-io/vine/service/context"
	"github.com/lack-io/vine/service/errors"
)

// srv is the service implementation of the Auth interface
type srv struct {
	options auth.Options
	auth    pb.AuthService
	rules   pb.RulesService
	token   token.Provider
}

func (s *srv) String() string {
	return "service"
}

func (s *srv) Init(opts ...auth.Option) {
	for _, o := range opts {
		o(&s.options)
	}
	s.auth = pb.NewAuthService("auth", client.DefaultClient)
	s.rules = pb.NewRulesService("auth", client.DefaultClient)
	s.setupJWT()
}

func (s *srv) Options() auth.Options {
	return s.options
}

// Generate a new account
func (s *srv) Generate(id string, opts ...auth.GenerateOption) (*auth.Account, error) {
	options := auth.NewGenerateOptions(opts...)
	if len(options.Issuer) == 0 {
		options.Issuer = s.options.Issuer
	}

	// we have the JWT private key and generate ourselves an account
	if len(s.options.PrivateKey) > 0 {
		acc := &auth.Account{
			ID:       id,
			Type:     options.Type,
			Scopes:   options.Scopes,
			Metadata: options.Metadata,
			Issuer:   options.Issuer,
		}

		tok, err := s.token.Generate(acc, token.WithExpiry(time.Hour*24*365))
		if err != nil {
			return nil, err
		}

		// when using JWTs, the account secret is the JWT's token. This
		// can be used as an argument in the Token method.
		acc.Secret = tok.Token
		return acc, nil
	}

	rsp, err := s.auth.Generate(context.DefaultContext, &pb.GenerateRequest{
		Id:       id,
		Type:     options.Type,
		Secret:   options.Secret,
		Scopes:   options.Scopes,
		Metadata: options.Metadata,
		Provider: options.Provider,
		Options: &pb.Options{
			Namespace: options.Issuer,
		},
		Name: options.Name,
	}, s.callOpts()...)
	if err != nil {
		return nil, err
	}

	return serializeAccount(rsp.Account), nil
}

// Grant access to a resource
func (s *srv) Grant(rule *auth.Rule) error {
	access := pb.Access_UNKNOWN
	if rule.Access == auth.AccessGranted {
		access = pb.Access_GRANTED
	} else if rule.Access == auth.AccessDenied {
		access = pb.Access_DENIED
	}

	_, err := s.rules.Create(context.DefaultContext, &pb.CreateRequest{
		Rule: &pb.Rule{
			Id:       rule.ID,
			Scope:    rule.Scope,
			Priority: rule.Priority,
			Access:   access,
			Resource: &pb.Resource{
				Type:     rule.Resource.Type,
				Name:     rule.Resource.Name,
				Endpoint: rule.Resource.Endpoint,
			},
		},
		Options: &pb.Options{
			Namespace: s.Options().Issuer,
		},
	}, s.callOpts()...)

	return err
}

// Revoke access to a resource
func (s *srv) Revoke(rule *auth.Rule) error {
	_, err := s.rules.Delete(context.DefaultContext, &pb.DeleteRequest{
		Id: rule.ID, Options: &pb.Options{
			Namespace: s.Options().Issuer,
		},
	}, s.callOpts()...)

	return err
}

func (s *srv) Rules(opts ...auth.RulesOption) ([]*auth.Rule, error) {
	var options auth.RulesOptions
	for _, o := range opts {
		o(&options)
	}
	if options.Context == nil {
		options.Context = context.DefaultContext
	}
	if len(options.Namespace) == 0 {
		options.Namespace = s.options.Issuer
	}

	callOpts := append(s.callOpts(), cache.CallExpiry(time.Second*30))
	rsp, err := s.rules.List(context.DefaultContext, &pb.ListRequest{
		Options: &pb.Options{Namespace: options.Namespace},
	}, callOpts...)
	if err != nil {
		return nil, err
	}

	rules := make([]*auth.Rule, len(rsp.Rules))
	for i, r := range rsp.Rules {
		rules[i] = serializeRule(r)
	}

	return rules, nil
}

// Verify an account has access to a resource
func (s *srv) Verify(acc *auth.Account, res *auth.Resource, opts ...auth.VerifyOption) error {
	var options auth.VerifyOptions
	for _, o := range opts {
		o(&options)
	}

	rs, err := s.Rules(
		auth.RulesContext(options.Context),
		auth.RulesNamespace(options.Namespace),
	)
	if err != nil {
		return err
	}

	return rules.VerifyAccess(rs, acc, res)
}

// Inspect a token
func (s *srv) Inspect(token string) (*auth.Account, error) {
	// validate the request
	if len(token) == 0 {
		return nil, auth.ErrInvalidToken
	}

	// try to decode JWT locally and fall back to srv if an error occurs
	if len(strings.Split(token, ".")) == 3 && len(s.options.PublicKey) > 0 {
		return s.token.Inspect(token)
	}

	// the token is not a JWT or we do not have the keys to decode it,
	// fall back to the auth service
	rsp, err := s.auth.Inspect(context.DefaultContext, &pb.InspectRequest{
		Token: token, Options: &pb.Options{Namespace: s.Options().Issuer},
	}, s.callOpts()...)
	if err != nil {
		return nil, err
	}
	return serializeAccount(rsp.Account), nil
}

// Token generation using an account ID and secret
func (s *srv) Token(opts ...auth.TokenOption) (*auth.AccountToken, error) {
	options := auth.NewTokenOptions(opts...)
	if len(options.Issuer) == 0 {
		options.Issuer = s.options.Issuer
	}

	tok := options.RefreshToken
	if len(options.Secret) > 0 {
		tok = options.Secret
	}

	// we have the JWT private key and refresh accounts locally
	if len(s.options.PrivateKey) > 0 && len(strings.Split(tok, ".")) == 3 {
		acc, err := s.token.Inspect(tok)
		if err != nil {
			return nil, err
		}

		token, err := s.token.Generate(acc, token.WithExpiry(options.Expiry))
		if err != nil {
			return nil, err
		}

		return &auth.AccountToken{
			Expiry:       token.Expiry,
			AccessToken:  token.Token,
			RefreshToken: tok,
		}, nil
	}

	rsp, err := s.auth.Token(context.DefaultContext, &pb.TokenRequest{
		Id:           options.ID,
		Secret:       options.Secret,
		RefreshToken: options.RefreshToken,
		TokenExpiry:  int64(options.Expiry.Seconds()),
		Options: &pb.Options{
			Namespace: options.Issuer,
		},
	}, s.callOpts()...)
	if err != nil && errors.FromErr(err).Detail == auth.ErrInvalidToken.Error() {
		return nil, auth.ErrInvalidToken
	} else if err != nil {
		return nil, err
	}

	return serializeToken(rsp.Token), nil
}

func serializeToken(t *pb.Token) *auth.AccountToken {
	return &auth.AccountToken{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		Created:      time.Unix(t.Created, 0),
		Expiry:       time.Unix(t.Expiry, 0),
	}
}

func serializeAccount(a *pb.Account) *auth.Account {
	return &auth.Account{
		ID:       a.Id,
		Secret:   a.Secret,
		Issuer:   a.Issuer,
		Metadata: a.Metadata,
		Scopes:   a.Scopes,
		Name:     a.Name,
	}
}

func serializeRule(r *pb.Rule) *auth.Rule {
	var access auth.Access
	if r.Access == pb.Access_GRANTED {
		access = auth.AccessGranted
	} else {
		access = auth.AccessDenied
	}

	return &auth.Rule{
		ID:       r.Id,
		Scope:    r.Scope,
		Access:   access,
		Priority: r.Priority,
		Resource: &auth.Resource{
			Type:     r.Resource.Type,
			Name:     r.Resource.Name,
			Endpoint: r.Resource.Endpoint,
		},
	}
}

func (s *srv) callOpts() []client.CallOption {
	return []client.CallOption{
		client.WithAddress(s.options.Addrs...),
		client.WithAuthToken(),
	}
}

// NewAuth returns a new instance of the Auth service
func NewAuth(opts ...auth.Option) auth.Auth {
	service := &srv{
		auth:    pb.NewAuthService("auth", client.DefaultClient),
		rules:   pb.NewRulesService("auth", client.DefaultClient),
		options: auth.NewOptions(opts...),
	}

	service.setupJWT()

	return service
}

func (s *srv) setupJWT() {
	tokenOpts := []token.Option{}

	// if we have a JWT public key passed as an option,
	// we can decode tokens with the type "JWT" locally
	// and not have to make an RPC call
	if key := s.options.PublicKey; len(key) > 0 {
		tokenOpts = append(tokenOpts, token.WithPublicKey(key))
	}

	// if we have a JWT private key passed as an option,
	// we can generate accounts locally and not have to make
	// an RPC call, this is used for vine clients such as
	// api, web, proxy.
	if key := s.options.PrivateKey; len(key) > 0 {
		tokenOpts = append(tokenOpts, token.WithPrivateKey(key))
	}

	s.token = jwt.NewTokenProvider(tokenOpts...)
}

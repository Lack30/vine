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

package auth

import (
	"context"
	"sync"

	"github.com/lack-io/vine/lib/dao"
	"github.com/lack-io/vine/lib/dao/sqlite"
	"golang.org/x/crypto/bcrypt"

	"github.com/lack-io/vine/lib/auth"
	"github.com/lack-io/vine/lib/auth/token"
	"github.com/lack-io/vine/lib/auth/token/basic"
	"github.com/lack-io/vine/lib/store"
	"github.com/lack-io/vine/proto/apis/errors"
	pb "github.com/lack-io/vine/proto/services/auth"
)

const (
	joinKey                  = "/"
	storePrefixAccounts      = "account"
	storePrefixRefreshTokens = "refresh"
)

var defaultAccount = &auth.Account{
	ID:     "default",
	Type:   "user",
	Scopes: []string{"admin"},
	Secret: "password",
}

// Auth processes RPC calls
type Auth struct {
	Options       auth.Options
	TokenProvider token.Provider

	namespaces map[string]bool
	sync.Mutex
}

// Init the auth
func (a *Auth) Init(opts ...auth.Option) {
	for _, o := range opts {
		o(&a.Options)
	}

	// use the default store as a fallback
	if a.Options.Dialect == nil {
		a.Options.Dialect = dao.DefaultDialect
	}

	// noop will not work for auth
	if a.Options.Dialect.String() == "noop" {
		a.Options.Dialect = sqlite.NewDialect()
	}

	// setup a token provider
	if a.TokenProvider == nil {
		a.TokenProvider = basic.NewTokenProvider(token.WithDialect(a.Options.Dialect))
	}
}

func (a *Auth) setupDefaultAccount(ns string) error {
	//a.Lock()
	//defer a.Unlock()
	//
	//// setup the namespace cache if not yet done
	//if a.namespaces == nil {
	//	a.namespaces = make(map[string]bool)
	//}
	//
	//// check to see if the default account has already been verified
	//if _, ok := a.namespaces[ns]; ok {
	//	return nil
	//}
	//
	//// setup a context with the namespace
	//ctx := namespace.ContextWithNamespace(context.TODO(), ns)
	//
	//// check to see if we need to create the default account
	//key := strings.Join([]string{storePrefixAccounts, ns, ""}, joinKey)
	//recs, err := a.Options.Store.Read(key, store.ReadPrefix())
	//if err != nil {
	//	return err
	//}
	//
	//hasUser := false
	//for _, rec := range recs {
	//	acc := &auth.Account{}
	//	err := json.Unmarshal(rec.Value, acc)
	//	if err != nil {
	//		return err
	//	}
	//	if acc.Type == "user" {
	//		hasUser = true
	//		break
	//	}
	//}
	//
	//// create the account if none exist in the namespace
	//if !hasUser {
	//	req := &pb.GenerateRequest{
	//		Id:     defaultAccount.ID,
	//		Type:   string(defaultAccount.Type),
	//		Scopes: defaultAccount.Scopes,
	//		Secret: defaultAccount.Secret,
	//	}
	//	log.Info("Generating default account")
	//	err = a.Generate(ctx, req, &pb.GenerateResponse{})
	//	if err != nil {
	//		return err
	//	}
	//}
	//
	//// set the namespace in the cache
	//a.namespaces[ns] = true
	return nil
}

// Generate an account
func (a *Auth) Generate(ctx context.Context, req *pb.GenerateRequest, rsp *pb.GenerateResponse) error {
	// validate the request
	//if len(req.Id) == 0 {
	//	return errors.BadRequest("go.vine.auth", "ID required")
	//}
	//
	//// set the defaults
	//if len(req.Type) == 0 {
	//	req.Type = "user"
	//}
	//if len(req.Secret) == 0 {
	//	req.Secret = uuid.New().String()
	//}
	//
	//// check the user does not already exists
	//key := strings.Join([]string{storePrefixAccounts, namespace.FromContext(ctx), req.Id}, joinKey)
	//if _, err := a.Options.Store.Read(key); err != store.ErrNotFound {
	//	return errors.BadRequest("go.vine.auth", "Account with this ID already exists")
	//}
	//
	//// hash the secret
	//secret, err := hashSecret(req.Secret)
	//if err != nil {
	//	return errors.InternalServerError("go.vine.auth", "Unable to hash password: %v", err)
	//}
	//
	//// Default to the current namespace as the scope. Once we add identity we can auto-generate
	//// these scopes and prevent users from accounts with any scope.
	//if len(req.Scopes) == 0 {
	//	req.Scopes = []string{"namespace." + namespace.FromContext(ctx)}
	//}
	//
	//// construct the account
	//acc := &auth.Account{
	//	ID:       req.Id,
	//	Type:     req.Type,
	//	Scopes:   req.Scopes,
	//	Metadata: req.Metadata,
	//	Issuer:   namespace.FromContext(ctx),
	//	Secret:   secret,
	//}
	//
	//// marshal to json
	//bytes, err := json.Marshal(acc)
	//if err != nil {
	//	return errors.InternalServerError("go.vine.auth", "Unable to marshal json: %v", err)
	//}
	//
	//// write to the store
	//if err := a.Options.Store.Write(&store.Record{Key: key, Value: bytes}); err != nil {
	//	return errors.InternalServerError("go.vine.auth", "Unable to write account to store: %v", err)
	//}
	//
	//// set a refresh token
	//if err := a.setRefreshToken(ctx, acc.ID, uuid.New().String()); err != nil {
	//	return errors.InternalServerError("go.vine.auth", "Unable to set a refresh token: %v", err)
	//}
	//
	//// return the account
	//rsp.Account = serializeAccount(acc)
	//rsp.Account.Secret = req.Secret // return unhashed secret
	return nil
}

// Inspect a token and retrieve the account
func (a *Auth) Inspect(ctx context.Context, req *pb.InspectRequest, rsp *pb.InspectResponse) error {
	acc, err := a.TokenProvider.Inspect(req.Token)
	if err == token.ErrInvalidToken || err == token.ErrNotFound {
		return errors.BadRequest("go.vine.auth", "Invalid token")
	} else if err != nil {
		return errors.InternalServerError("go.vine.auth", "Unable to inspect token: %v", err)
	}

	rsp.Account = serializeAccount(acc)
	return nil
}

// Token generation using an account ID and secret
func (a *Auth) Token(ctx context.Context, req *pb.TokenRequest, rsp *pb.TokenResponse) error {
	// setup the defaults incase none exist
	//err := a.setupDefaultAccount(namespace.FromContext(ctx))
	//if err != nil {
	//	// failing gracefully here
	//	log.Errorf("Error setting up default accounts: %v", err)
	//}
	//
	//// validate the request
	//if (len(req.Id) == 0 || len(req.Secret) == 0) && len(req.RefreshToken) == 0 {
	//	return errors.BadRequest("go.vine.auth", "Credentials or a refresh token required")
	//}
	//
	//// Declare the account id and refresh token
	//accountID := req.Id
	//refreshToken := req.RefreshToken
	//
	//// If the refresh token is set, check this
	//if len(req.RefreshToken) > 0 {
	//	accID, err := a.accountIDForRefreshToken(ctx, req.RefreshToken)
	//	if err == store.ErrNotFound {
	//		return errors.BadRequest("go.vine.auth", "Invalid token")
	//	} else if err != nil {
	//		return errors.InternalServerError("go.vine.auth", "Unable to lookup token: %v", err)
	//	}
	//	accountID = accID
	//}
	//
	//// Lookup the account in the store
	//key := strings.Join([]string{storePrefixAccounts, namespace.FromContext(ctx), accountID}, joinKey)
	//recs, err := a.Options.Store.Read(key)
	//if err == store.ErrNotFound {
	//	return errors.BadRequest("go.vine.auth", "Account not found with this ID")
	//} else if err != nil {
	//	return errors.InternalServerError("go.vine.auth", "Unable to read from store: %v", err)
	//}
	//
	//// Unmarshal the record
	//var acc *auth.Account
	//if err := json.Unmarshal(recs[0].Value, &acc); err != nil {
	//	return errors.InternalServerError("go.vine.auth", "Unable to unmarshal account: %v", err)
	//}
	//
	//// If the refresh token was not used, validate the secrets match and then set the refresh token
	//// so it can be returned to the user
	//if len(req.RefreshToken) == 0 {
	//	if !secretsMatch(acc.Secret, req.Secret) {
	//		return errors.BadRequest("go.vine.auth", "Secret not correct")
	//	}
	//
	//	refreshToken, err = a.refreshTokenForAccount(ctx, acc.ID)
	//	if err != nil {
	//		return errors.InternalServerError("go.vine.auth", "Unable to get refresh token: %v", err)
	//	}
	//}
	//
	//// Generate a new access token
	//duration := time.Duration(req.TokenExpiry) * time.Second
	//tok, err := a.TokenProvider.Generate(acc, token.WithExpiry(duration))
	//if err != nil {
	//	return errors.InternalServerError("go.vine.auth", "Unable to generate token: %v", err)
	//}
	//
	//rsp.Token = serializeToken(tok, refreshToken)
	return nil
}

// set the refresh token for an account
func (a *Auth) setRefreshToken(ctx context.Context, id, token string) error {
	//key := strings.Join([]string{storePrefixRefreshTokens, namespace.FromContext(ctx), id, token}, joinKey)
	//return a.Options.Store.Write(&store.Record{Key: key})
	return nil
}

// get the refresh token for an accutn
func (a *Auth) refreshTokenForAccount(ctx context.Context, id string) (string, error) {
	//ns := namespace.FromContext(ctx)
	//prefix := strings.Join([]string{storePrefixRefreshTokens, ns, id, ""}, joinKey)
	//
	//recs, err := a.Options.Store.Read(prefix, store.ReadPrefix())
	//if err != nil {
	//	return "", err
	//} else if len(recs) == 0 {
	//	return "", store.ErrNotFound
	//}
	//
	//comps := strings.Split(recs[0].Key, "/")
	//if len(comps) != 4 {
	//	return "", store.ErrNotFound
	//}
	//return comps[3], nil
	return "", nil
}

// get the account ID for the given refresh token
func (a *Auth) accountIDForRefreshToken(ctx context.Context, token string) (string, error) {
	//prefix := strings.Join([]string{storePrefixRefreshTokens, namespace.FromContext(ctx)}, joinKey)
	//keys, err := a.Options.Store.List(store.ListPrefix(prefix))
	//if err != nil {
	//	return "", err
	//}
	//
	//for _, k := range keys {
	//	if strings.HasSuffix(k, "/"+token) {
	//		comps := strings.Split(k, "/")
	//		if len(comps) != 4 {
	//			return "", store.ErrNotFound
	//		}
	//		return comps[2], nil
	//	}
	//}

	return "", store.ErrNotFound
}

func (a *Auth) Verify(ctx context.Context, request *pb.VerifyRequest, response *pb.VerifyResponse) error {
	panic("implement me")
}

func serializeToken(t *token.Token, refresh string) *pb.Token {
	return &pb.Token{
		Created:      t.Created.Unix(),
		Expiry:       t.Expiry.Unix(),
		AccessToken:  t.Token,
		RefreshToken: refresh,
	}
}

func hashSecret(s string) (string, error) {
	saltedBytes := []byte(s)
	hashedBytes, err := bcrypt.GenerateFromPassword(saltedBytes, bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	hash := string(hashedBytes[:])
	return hash, nil
}

func secretsMatch(hash string, s string) bool {
	incoming := []byte(s)
	existing := []byte(hash)
	return bcrypt.CompareHashAndPassword(existing, incoming) == nil
}

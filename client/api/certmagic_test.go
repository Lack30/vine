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

package api

import (
	"net"
	"net/http"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/go-acme/lego/v3/providers/dns/cloudflare"

	"github.com/lack-io/vine/service/api/server/acme"
	cert "github.com/lack-io/vine/service/api/server/acme/certmagic"
	"github.com/lack-io/vine/service/sync/memory"
	cfstore "github.com/lack-io/vine/util/plugins/store/cloudflare"
)

func TestCertMagic(t *testing.T) {
	if len(os.Getenv("IN_TRAVIS_CI")) != 0 {
		t.Skip()
	}
	l, err := cert.NewProvider().Listen()
	if err != nil {
		if _, ok := err.(*net.OpError); ok {
			t.Skip("Run under non privileged user")
		}
		t.Fatal(err.Error())
	}
	l.Close()

	c := cloudflare.NewDefaultConfig()
	c.AuthEmail = ""
	c.AuthKey = ""
	c.AuthToken = "test"
	c.ZoneToken = "test"

	p, err := cloudflare.NewDNSProviderConfig(c)
	if err != nil {
		t.Fatal(err.Error())
	}

	l, err = cert.NewProvider(acme.AcceptToS(true),
		acme.CA(acme.LetsEncryptStagingCA),
		acme.ChallengeProvider(p),
	).Listen()

	if err != nil {
		t.Fatal(err.Error())
	}
	l.Close()
}

func TestStorageImplementation(t *testing.T) {
	if len(os.Getenv("IN_TRAVIS_CI")) != 0 {
		t.Skip()
	}

	apiToken, accountID := os.Getenv("CF_API_TOKEN"), os.Getenv("CF_ACCOUNT_ID")
	kvID := os.Getenv("KV_NAMESPACE_ID")
	if len(apiToken) == 0 || len(accountID) == 0 || len(kvID) == 0 {
		t.Skip("No Cloudflare API keys available, skipping test")
	}

	st := cfstore.NewStore(
		cfstore.Token(apiToken),
		cfstore.Account(accountID),
		cfstore.Namespace(kvID),
	)
	s := cert.NewStorage(memory.NewSync(), st)

	// Test Lock
	if err := s.Lock("test"); err != nil {
		t.Fatal(err)
	}

	// Test Unlock
	if err := s.Unlock("test"); err != nil {
		t.Fatal(err)
	}

	// Test data
	testdata := []struct {
		key   string
		value []byte
	}{
		{key: "/foo/a", value: []byte("lorem")},
		{key: "/foo/b", value: []byte("ipsum")},
		{key: "/foo/c", value: []byte("dolor")},
		{key: "/foo/d", value: []byte("sit")},
		{key: "/bar/a", value: []byte("amet")},
		{key: "/bar/b", value: []byte("consectetur")},
		{key: "/bar/c", value: []byte("adipiscing")},
		{key: "/bar/d", value: []byte("elit")},
		{key: "/foo/bar/a", value: []byte("sed")},
		{key: "/foo/bar/b", value: []byte("do")},
		{key: "/foo/bar/c", value: []byte("eiusmod")},
		{key: "/foo/bar/d", value: []byte("tempor")},
		{key: "/foo/bar/baz/a", value: []byte("incididunt")},
		{key: "/foo/bar/baz/b", value: []byte("ut")},
		{key: "/foo/bar/baz/c", value: []byte("labore")},
		{key: "/foo/bar/baz/d", value: []byte("et")},
		// a duplicate just in case there's any edge cases
		{key: "/foo/a", value: []byte("lorem")},
	}

	// Test Store
	for _, d := range testdata {
		if err := s.Store(d.key, d.value); err != nil {
			t.Fatal(err.Error())
		}
	}

	// Test Load
	for _, d := range testdata {
		if value, err := s.Load(d.key); err != nil {
			t.Fatal(err.Error())
		} else {
			if !reflect.DeepEqual(value, d.value) {
				t.Fatalf("Load %s: expected %v, got %v", d.key, d.value, value)
			}
		}
	}

	// Test Exists
	for _, d := range testdata {
		if !s.Exists(d.key) {
			t.Fatalf("%s should exist, but doesn't\n", d.key)
		}
	}

	// Test List
	if list, err := s.List("/", true); err != nil {
		t.Fatal(err.Error())
	} else {
		var expected []string
		for i, d := range testdata {
			if i != len(testdata)-1 {
				// Don't store the intentionally duplicated key
				expected = append(expected, d.key)
			}
		}
		sort.Strings(expected)
		sort.Strings(list)
		if !reflect.DeepEqual(expected, list) {
			t.Fatalf("List: Expected %v, got %v\n", expected, list)
		}
	}
	if list, err := s.List("/foo", false); err != nil {
		t.Fatal(err.Error())
	} else {
		sort.Strings(list)
		expected := []string{"/foo/a", "/foo/b", "/foo/bar", "/foo/c", "/foo/d"}
		if !reflect.DeepEqual(expected, list) {
			t.Fatalf("List: expected %s, got %s\n", expected, list)
		}
	}

	// Test Stat
	for _, d := range testdata {
		info, err := s.Stat(d.key)
		if err != nil {
			t.Fatal(err.Error())
		} else {
			if info.Key != d.key {
				t.Fatalf("Stat().Key: expected %s, got %s\n", d.key, info.Key)
			}
			if info.Size != int64(len(d.value)) {
				t.Fatalf("Stat().Size: expected %d, got %d\n", len(d.value), info.Size)
			}
			if time.Since(info.Modified) > time.Minute {
				t.Fatalf("Stat().Modified: expected time since last modified to be < 1 minute, got %v\n", time.Since(info.Modified))
			}
		}

	}

	// Test Delete
	for _, d := range testdata {
		if err := s.Delete(d.key); err != nil {
			t.Fatal(err.Error())
		}
	}

	// New interface doesn't return an error, so call it in case any log.Fatal
	// happens
	cert.NewProvider(acme.Cache(s))
}

// Full test with a real zone, with  against LE staging
func TestE2e(t *testing.T) {
	if len(os.Getenv("IN_TRAVIS_CI")) != 0 {
		t.Skip()
	}

	apiToken, accountID := os.Getenv("CF_API_TOKEN"), os.Getenv("CF_ACCOUNT_ID")
	kvID := os.Getenv("KV_NAMESPACE_ID")
	if len(apiToken) == 0 || len(accountID) == 0 || len(kvID) == 0 {
		t.Skip("No Cloudflare API keys available, skipping test")
	}

	testLock := memory.NewSync()
	testStore := cfstore.NewStore(
		cfstore.Token(apiToken),
		cfstore.Account(accountID),
		cfstore.Namespace(kvID),
	)
	testStorage := cert.NewStorage(testLock, testStore)

	conf := cloudflare.NewDefaultConfig()
	conf.AuthToken = apiToken
	conf.ZoneToken = apiToken
	testChallengeProvider, err := cloudflare.NewDNSProviderConfig(conf)
	if err != nil {
		t.Fatal(err.Error())
	}

	testProvider := cert.NewProvider(
		acme.AcceptToS(true),
		acme.Cache(testStorage),
		acme.CA(acme.LetsEncryptStagingCA),
		acme.ChallengeProvider(testChallengeProvider),
		acme.OnDemand(false),
	)

	listener, err := testProvider.Listen("*.vine.mu", "vine.mu")
	if err != nil {
		t.Fatal(err.Error())
	}
	go http.Serve(listener, http.NotFoundHandler())
	time.Sleep(10 * time.Minute)
}

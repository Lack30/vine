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

// Package web is a web dashboard
package web

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gorilla/mux"
	"github.com/lack-io/cli"
	"github.com/serenize/snaker"
	"golang.org/x/net/publicsuffix"

	"github.com/lack-io/vine"
	"github.com/lack-io/vine/cmd/vine/client/resolver/web"
	regpb "github.com/lack-io/vine/proto/apis/registry"
	res "github.com/lack-io/vine/service/api/resolver"
	"github.com/lack-io/vine/service/api/server/cors"
	"github.com/lack-io/vine/service/auth"
	log "github.com/lack-io/vine/service/logger"
	"github.com/lack-io/vine/service/registry"
	"github.com/lack-io/vine/util/namespace"
)

//Meta Fields of vine web
var (
	// Default server name
	Name = "go.vine.web"
	// Default address to bind to
	Address = ":8082"
	// The namespace to serve
	// Example:
	// Namespace + /[Service]/foo/bar
	// Host: Namespace.Service Endpoint: /foo/bar
	Namespace = "go.vine"
	Type      = "web"
	Resolver  = "path"
	// Base path sent to web service.
	// This is stripped from the request path
	// Allows the web service to define absolute paths
	BasePathHeader = "X-Vine-Web-Base-Path"
	statsURL       string
	loginURL       string

	// Host name the web dashboard is served on
	Host, _ = os.Hostname()
)

type service struct {
	*mux.Router
	// registry we use
	registry registry.Registry
	// the resolver
	resolver *web.Resolver
	// the namespace resolver
	nsResolver *namespace.Resolver
	// the proxy server
	prx *proxy
	// auth service
	auth auth.Auth
}

type reg struct {
	registry.Registry

	sync.RWMutex
	lastPull time.Time
	services []*regpb.Service
}

// ServeHTTP serves the web dashboard and proxies where appropriate
func (s *service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Host) == 0 {
		r.URL.Host = r.Host
	}

	if len(r.URL.Scheme) == 0 {
		r.URL.Scheme = "http"
	}

	// no host means dashboard
	host := r.URL.Hostname()
	if len(host) == 0 {
		h, _, err := net.SplitHostPort(r.Host)
		if err != nil && strings.Contains(err.Error(), "missing port in address") {
			host = r.Host
		} else if err == nil {
			host = h
		}
	}

	// check again
	if len(host) == 0 {
		s.Router.ServeHTTP(w, r)
		return
	}

	// check based on host set
	if len(Host) > 0 && Host == host {
		s.Router.ServeHTTP(w, r)
		return
	}

	// an ip instead of hostname means dashboard
	ip := net.ParseIP(host)
	if ip != nil {
		s.Router.ServeHTTP(w, r)
		return
	}

	// namespace matching host means dashboard
	parts := strings.Split(host, ".")
	reverse(parts)
	namespace := strings.Join(parts, ".")

	// replace mu since we know its ours
	if strings.HasPrefix(namespace, "mu.vine") {
		namespace = strings.Replace(namespace, "mu.vine", "go.vine", 1)
	}

	// web dashboard if namespace matches
	if namespace == Namespace+"."+Type {
		s.Router.ServeHTTP(w, r)
		return
	}

	// if a host has no subdomain serve dashboard
	v, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil || v == host {
		s.Router.ServeHTTP(w, r)
		return
	}

	// check if its a web request
	if _, _, isWeb := s.resolver.Info(r); isWeb {
		s.Router.ServeHTTP(w, r)
		return
	}

	// otherwise serve the proxy
	s.prx.ServeHTTP(w, r)
}

// proxy is a http reverse proxy
func (s *service) proxy() *proxy {
	director := func(r *http.Request) {
		kill := func() {
			r.URL.Host = ""
			r.URL.Path = ""
			r.URL.Scheme = ""
			r.Host = ""
			r.RequestURI = ""
		}

		// check to see if the endpoint was encoded in the request context
		// by the auth wrapper
		var endpoint *res.Endpoint
		if val, ok := (r.Context().Value(res.Endpoint{})).(*res.Endpoint); ok {
			endpoint = val
		}

		// TODO: better error handling
		var err error
		if endpoint == nil {
			if endpoint, err = s.resolver.Resolve(r); err != nil {
				log.Errorf("Failed to resolve url: %v: %v\n", r.URL, err)
				kill()
				return
			}
		}

		r.Header.Set(BasePathHeader, "/"+endpoint.Name)
		r.URL.Host = endpoint.Host
		r.URL.Path = endpoint.Path
		r.URL.Scheme = "http"
		r.Host = r.URL.Host
	}

	return &proxy{
		Router:   &httputil.ReverseProxy{Director: director},
		Director: director,
	}
}

func format(v *regpb.Value) string {
	if v == nil || len(v.Values) == 0 {
		return "{}"
	}
	var f []string
	for _, k := range v.Values {
		f = append(f, formatEndpoint(k, 0))
	}
	return fmt.Sprintf("{\n%s}", strings.Join(f, ""))
}

func formatEndpoint(v *regpb.Value, r int) string {
	// default format is tabbed plus the value plus new line
	fparts := []string{"", "%s %s", "\n"}
	for i := 0; i < r+1; i++ {
		fparts[0] += "\t"
	}
	// its just a primitive of sorts so return
	if len(v.Values) == 0 {
		return fmt.Sprintf(strings.Join(fparts, ""), snaker.CamelToSnake(v.Name), v.Type)
	}

	// this thing has more things, it's complex
	fparts[1] += " {"

	vals := []interface{}{snaker.CamelToSnake(v.Name), v.Type}

	for _, val := range v.Values {
		fparts = append(fparts, "%s")
		vals = append(vals, formatEndpoint(val, r+1))
	}

	// at the end
	l := len(fparts) - 1
	for i := 0; i < r+1; i++ {
		fparts[l] += "\t"
	}
	fparts = append(fparts, "}\n")

	return fmt.Sprintf(strings.Join(fparts, ""), vals...)
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	return
}

func (s *service) indexHandler(c *fiber.Ctx) error {
	cors.SetHeaders(c)

	if c.Method() == "OPTIONS" {
		return nil
	}

	services, err := s.registry.ListServices(registry.ListContext(c.Context()))
	if err != nil {
		log.Errorf("Error listing services: %v", err)
	}

	type webService struct {
		Name string
		Link string
		Icon string // TODO: lookup icon
	}

	// if the resolver is subdomain, we will need the domain
	domain, _ := publicsuffix.EffectiveTLDPlusOne(c.Hostname())

	var webServices []webService
	for _, svc := range services {
		// not a web app
		comps := strings.Split(svc.Name, ".web.")
		if len(comps) == 1 {
			continue
		}
		name := comps[1]

		link := fmt.Sprintf("/%v/", name)
		if Resolver == "subdomain" && len(domain) > 0 {
			link = fmt.Sprintf("https://%v.%v", name, domain)
		}

		// in the case of 3 letter things e.g m3o convert to M3O
		if len(name) <= 3 && strings.ContainsAny(name, "012345789") {
			name = strings.ToUpper(name)
		}

		webServices = append(webServices, webService{Name: name, Link: link})
	}

	sort.Slice(webServices, func(i, j int) bool { return webServices[i].Name < webServices[j].Name })

	type templateData struct {
		HasWebServices bool
		WebServices    []webService
	}

	data := templateData{len(webServices) > 0, webServices}
	return s.render(c, indexTemplate, data)
}

func (s *service) registryHandler(c *fiber.Ctx) error {
	//vars := mux.Vars(c)
	//svc := vars["name"]
	//
	//if len(svc) > 0 {
	//	sv, err := s.registry.GetService(svc, registry.GetContext(r.Context()))
	//	if err != nil {
	//		http.Error(w, "Error occurred:"+err.Error(), 500)
	//		return
	//	}
	//
	//	if len(sv) == 0 {
	//		http.Error(w, "Not found", 404)
	//		return
	//	}
	//
	//	if r.Header.Get("Content-Type") == "application/json" {
	//		b, err := json.Marshal(map[string]interface{}{
	//			"services": s,
	//		})
	//		if err != nil {
	//			http.Error(w, "Error occurred:"+err.Error(), 500)
	//			return
	//		}
	//		w.Header().Set("Content-Type", "application/json")
	//		w.Write(b)
	//		return
	//	}
	//
	//	s.render(c, serviceTemplate, sv)
	//	return
	//}
	//
	//services, err := s.registry.ListServices(registry.ListContext(r.Context()))
	//if err != nil {
	//	log.Errorf("Error listing services: %v", err)
	//}
	//
	//sort.Sort(sortedServices{services})
	//
	//if r.Header.Get("Content-Type") == "application/json" {
	//	b, err := json.Marshal(map[string]interface{}{
	//		"services": services,
	//	})
	//	if err != nil {
	//		http.Error(w, "Error occurred:"+err.Error(), 500)
	//		return
	//	}
	//	w.Header().Set("Content-Type", "application/json")
	//	w.Write(b)
	//	return
	//}

	//return s.render(c, registryTemplate, services)
	return nil
}

func (s *service) callHandler(c *fiber.Ctx) error {
	//services, err := s.registry.ListServices(registry.ListContext(c.Context()))
	//if err != nil {
	//	log.Errorf("Error listing services: %v", err)
	//}
	//
	//sort.Sort(sortedServices{services})
	//
	//serviceMap := make(map[string][]*regpb.Endpoint)
	//for _, service := range services {
	//	if len(service.Endpoints) > 0 {
	//		serviceMap[service.Name] = service.Endpoints
	//		continue
	//	}
	//	// lookup the endpoints otherwise
	//	s, err := s.registry.GetService(service.Name, registry.GetContext(r.Context()))
	//	if err != nil {
	//		continue
	//	}
	//	if len(s) == 0 {
	//		continue
	//	}
	//	serviceMap[service.Name] = s[0].Endpoints
	//}
	//
	//if r.Header.Get("Content-Type") == "application/json" {
	//	b, err := json.Marshal(map[string]interface{}{
	//		"services": services,
	//	})
	//	if err != nil {
	//		http.Error(w, "Error occurred:"+err.Error(), 500)
	//		return
	//	}
	//	w.Header().Set("Content-Type", "application/json")
	//	w.Write(b)
	//	return
	//}
	//
	//return s.render(c, callTemplate, serviceMap)
	return nil
}

func (s *service) render(c *fiber.Ctx, tmpl string, data interface{}) error {
	//t, err := template.New("template").Funcs(template.FuncMap{
	//	"format": format,
	//	"Title":  strings.Title,
	//	"First": func(s string) string {
	//		if len(s) == 0 {
	//			return s
	//		}
	//		return strings.Title(string(s[0]))
	//	},
	//}).Parse(layoutTemplate)
	//if err != nil {
	//	http.Error(w, "Error occurred:"+err.Error(), 500)
	//	return
	//}
	//t, err = t.Parse(tmpl)
	//if err != nil {
	//	http.Error(w, "Error occurred:"+err.Error(), 500)
	//	return
	//}
	//
	//// If the user is logged in, render Account instead of Login
	//loginTitle := "Login"
	//user := ""
	//
	//if c, err := r.Cookie(inauth.TokenCookieName); err == nil && c != nil {
	//	token := strings.TrimPrefix(c.Value, inauth.TokenCookieName+"=")
	//	if acc, err := s.auth.Inspect(token); err == nil {
	//		loginTitle = "Account"
	//		user = acc.ID
	//	}
	//}
	//
	//if err := t.ExecuteTemplate(w, "layout", map[string]interface{}{
	//	"LoginTitle": loginTitle,
	//	"LoginURL":   loginURL,
	//	"StatsURL":   statsURL,
	//	"Results":    data,
	//	"User":       user,
	//}); err != nil {
	//	http.Error(w, "Error occurred:"+err.Error(), 500)
	//}
	return nil
}

func Run(ctx *cli.Context, svcOpts ...vine.Option) {

	//if len(ctx.String("server-name")) > 0 {
	//	Name = ctx.String("server-name")
	//}
	//if len(ctx.String("address")) > 0 {
	//	Address = ctx.String("address")
	//}
	//if len(ctx.String("resolver")) > 0 {
	//	Resolver = ctx.String("resolver")
	//}
	//if len(ctx.String("type")) > 0 {
	//	Type = ctx.String("type")
	//}
	//if len(ctx.String("namespace")) > 0 {
	//	// remove the service type from the namespace to allow for
	//	// backwards compatability
	//	Namespace = strings.TrimSuffix(ctx.String("namespace"), "."+Type)
	//}
	//
	//// service opts
	//svcOpts = append(svcOpts, vine.Name(Name))
	//
	//// Initialize Server
	//svc := vine.NewService(svcOpts...)
	//
	//reg := &reg{Registry: *cmd.DefaultOptions().Registry}
	//
	//s := &service{
	//	Router:   mux.NewRouter(),
	//	registry: reg,
	//	// our internal resolver
	//	resolver: &web.Resolver{
	//		// Default to type path
	//		Type:      Resolver,
	//		Namespace: namespace.NewResolver(Type, Namespace).ResolveWithType,
	//		Selector: selector.NewSelector(
	//			selector.Registry(reg),
	//		),
	//	},
	//	auth: *cmd.DefaultOptions().Auth,
	//}
	//
	//var h http.Handler
	//// set as the server
	//h = s
	//
	//if ctx.Bool("enable-stats") {
	//	statsURL = "/stats"
	//	st := stats.New()
	//	s.HandleFunc("/stats", st.StatsHandler)
	//	h = st.ServeHTTP(s)
	//	st.Start()
	//	defer st.Stop()
	//}
	//
	//// create the proxy
	//p := s.proxy()
	//
	//// the web handler itself
	//s.HandleFunc("/favicon.ico", faviconHandler)
	//s.HandleFunc("/client", s.callHandler)
	//s.HandleFunc("/services", s.registryHandler)
	//s.HandleFunc("/service/{name}", s.registryHandler)
	//s.HandleFunc("/rpc", handler.RPC)
	//s.PathPrefix("/{service:[a-zA-Z0-9]+}").Handler(p)
	//s.HandleFunc("/", s.indexHandler)
	//
	//// insert the proxy
	//s.prx = p
	//
	//var opts []server.Option
	//
	//if ctx.Bool("enable-tls") {
	//	config, err := helper.TLSConfig(ctx)
	//	if err != nil {
	//		log.Errorf(err.Error())
	//		return
	//	}
	//
	//	opts = append(opts, server.EnableTLS(true))
	//	opts = append(opts, server.TLSConfig(config))
	//}
	//
	//// create the namespace resolver and the auth wrapper
	//s.nsResolver = namespace.NewResolver(Type, Namespace)
	//authWrapper := apiAuth.Wrapper(s.resolver, s.nsResolver)
	//
	//// create the service and add the auth wrapper
	//server := httpapi.NewServer(Address, server.WrapHandler(authWrapper))
	//
	//server.Init(opts...)
	//server.Handle("/", h)
	//
	//// Setup auth redirect
	//if len(ctx.String("auth-login-url")) > 0 {
	//	loginURL = ctx.String("auth-login-url")
	//	svc.Options().Auth.Init(auth.LoginURL(loginURL))
	//}
	//
	//if err := server.Start(); err != nil {
	//	log.Fatal(err)
	//}
	//
	//// Run server
	//if err := svc.Run(); err != nil {
	//	log.Fatal(err)
	//}
	//
	//if err := server.Stop(); err != nil {
	//	log.Fatal(err)
	//}
}

//Commands for `vine web`
func Commands(options ...vine.Option) []*cli.Command {
	command := &cli.Command{
		Name:  "web",
		Usage: "Run the web dashboard",
		Action: func(c *cli.Context) error {
			Run(c, options...)
			return nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "address",
				Usage:   "Set the web UI address e.g 0.0.0.0:8082",
				EnvVars: []string{"VINE_WEB_ADDRESS"},
			},
			&cli.StringFlag{
				Name:    "namespace",
				Usage:   "Set the namespace used by the Web proxy e.g. com.example.web",
				EnvVars: []string{"VINE_WEB_NAMESPACE"},
			},
			&cli.StringFlag{
				Name:    "resolver",
				Usage:   "Set the resolver to route to services e.g path, domain",
				EnvVars: []string{"VINE_WEB_RESOLVER"},
			},
			&cli.StringFlag{
				Name:    "auth-login-url",
				EnvVars: []string{"VINE_AUTH_LOGIN_URL"},
				Usage:   "The relative URL where a user can login",
			},
		},
	}

	return []*cli.Command{command}
}

func reverse(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

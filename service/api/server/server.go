// Package api is an API Gateway
package api

import (
	"fmt"
	"net/http"
	"os"

	"github.com/go-acme/lego/v3/providers/dns/cloudflare"
	"github.com/gorilla/mux"
	"github.com/lack-io/cli"

	"github.com/lack-io/vine/client"
	ahandler "github.com/lack-io/vine/internal/api/handler"
	aapi "github.com/lack-io/vine/internal/api/handler/api"
	"github.com/lack-io/vine/internal/api/handler/event"
	ahttp "github.com/lack-io/vine/internal/api/handler/http"
	arpc "github.com/lack-io/vine/internal/api/handler/rpc"
	"github.com/lack-io/vine/internal/api/handler/web"
	"github.com/lack-io/vine/internal/api/resolver"
	"github.com/lack-io/vine/internal/api/resolver/grpc"
	"github.com/lack-io/vine/internal/api/resolver/host"
	"github.com/lack-io/vine/internal/api/resolver/path"
	"github.com/lack-io/vine/internal/api/resolver/subdomain"
	"github.com/lack-io/vine/internal/api/router"
	regRouter "github.com/lack-io/vine/internal/api/router/registry"
	"github.com/lack-io/vine/internal/api/server"
	"github.com/lack-io/vine/internal/api/server/acme"
	"github.com/lack-io/vine/internal/api/server/acme/autocert"
	"github.com/lack-io/vine/internal/api/server/acme/certmagic"
	httpapi "github.com/lack-io/vine/internal/api/server/http"
	"github.com/lack-io/vine/internal/handler"
	"github.com/lack-io/vine/internal/helper"
	rrvine "github.com/lack-io/vine/internal/resolver/api"
	"github.com/lack-io/vine/internal/sync/memory"
	"github.com/lack-io/vine/plugin"
	"github.com/lack-io/vine/service"
	"github.com/lack-io/vine/service/api/auth"
	log "github.com/lack-io/vine/service/logger"
	muregistry "github.com/lack-io/vine/service/registry"
	"github.com/lack-io/vine/service/store"
)

var (
	Name                  = "api"
	Address               = ":8080"
	Handler               = "meta"
	Resolver              = "vine"
	APIPath               = "/"
	ProxyPath             = "/{service:[a-zA-Z0-9]+}"
	Namespace             = ""
	ACMEProvider          = "autocert"
	ACMEChallengeProvider = "cloudflare"
	ACMECA                = acme.LetsEncryptProductionCA
)

var (
	Flags = append(client.Flags,
		&cli.StringFlag{
			Name:    "address",
			Usage:   "Set the api address e.g 0.0.0.0:8080",
			EnvVars: []string{"VINE_API_ADDRESS"},
		},
		&cli.StringFlag{
			Name:    "handler",
			Usage:   "Specify the request handler to be used for mapping HTTP requests to services; {api, event, http, rpc}",
			EnvVars: []string{"VINE_API_HANDLER"},
		},
		&cli.StringFlag{
			Name:    "namespace",
			Usage:   "Set the namespace used by the API e.g. com.example",
			EnvVars: []string{"VINE_API_NAMESPACE"},
		},
		&cli.StringFlag{
			Name:    "resolver",
			Usage:   "Set the hostname resolver used by the API {host, path, grpc}",
			EnvVars: []string{"VINE_API_RESOLVER"},
		},
		&cli.BoolFlag{
			Name:    "enable_cors",
			Usage:   "Enable CORS, allowing the API to be called by frontend applications",
			EnvVars: []string{"VINE_API_ENABLE_CORS"},
			Value:   true,
		},
	)
)

func Run(ctx *cli.Context) error {
	if len(ctx.String("server-name")) > 0 {
		Name = ctx.String("server-name")
	}
	if len(ctx.String("address")) > 0 {
		Address = ctx.String("address")
	}
	if len(ctx.String("handler")) > 0 {
		Handler = ctx.String("handler")
	}
	if len(ctx.String("resolver")) > 0 {
		Resolver = ctx.String("resolver")
	}
	if len(ctx.String("acme-provider")) > 0 {
		ACMEProvider = ctx.String("acme-provider")
	}
	if len(ctx.String("namespace")) > 0 {
		Namespace = ctx.String("namespace")
	}
	if len(ctx.String("api-handler")) > 0 {
		Handler = ctx.String("api-handler")
	}
	if len(ctx.String("api-address")) > 0 {
		Address = ctx.String("api-address")
	}
	// initialise service
	srv := service.New(service.Name(Name))

	// Init API
	var opts []server.Option

	if ctx.Bool("enable-acme") {
		hosts := helper.ACMEHosts(ctx)
		opts = append(opts, server.EnableACME(true))
		opts = append(opts, server.ACMEHosts(hosts...))
		switch ACMEProvider {
		case "autocert":
			opts = append(opts, server.ACMEProvider(autocert.NewProvider()))
		case "certmagic":
			if ACMEChallengeProvider != "cloudflare" {
				log.Fatal("The only implemented DNS challenge provider is cloudflare")
			}

			apiToken := os.Getenv("CF_API_TOKEN")
			if len(apiToken) == 0 {
				log.Fatal("env variables CF_API_TOKEN and CF_ACCOUNT_ID must be set")
			}

			storage := certmagic.NewStorage(
				memory.NewSync(),
				store.DefaultStore,
			)

			config := cloudflare.NewDefaultConfig()
			config.AuthToken = apiToken
			config.ZoneToken = apiToken
			challengeProvider, err := cloudflare.NewDNSProviderConfig(config)
			if err != nil {
				log.Fatal(err.Error())
			}

			opts = append(opts,
				server.ACMEProvider(
					certmagic.NewProvider(
						acme.AcceptToS(true),
						acme.CA(ACMECA),
						acme.Cache(storage),
						acme.ChallengeProvider(challengeProvider),
						acme.OnDemand(false),
					),
				),
			)
		default:
			log.Fatalf("%s is not a valid ACME provider\n", ACMEProvider)
		}
	} else if ctx.Bool("enable-tls") {
		config, err := helper.TLSConfig(ctx)
		if err != nil {
			fmt.Println(err.Error())
			return err
		}

		opts = append(opts, server.EnableTLS(true))
		opts = append(opts, server.TLSConfig(config))
	}

	if ctx.Bool("enable-cors") {
		opts = append(opts, server.EnableCORS(true))
	}

	// create the router
	var h http.Handler
	r := mux.NewRouter()
	h = r

	// return version and list of services
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			return
		}

		response := fmt.Sprintf(`{"version": "%s"}`, ctx.App.Version)
		w.Write([]byte(response))
	})

	// strip favicon.ico
	r.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {})

	// resolver options
	ropts := []resolver.Option{
		resolver.WithServicePrefix(Namespace),
		resolver.WithHandler(Handler),
	}

	// default resolver
	rr := rrvine.NewResolver(ropts...)

	switch Resolver {
	case "subdomain":
		rr = subdomain.NewResolver(rr)
	case "host":
		rr = host.NewResolver(ropts...)
	case "path":
		rr = path.NewResolver(ropts...)
	case "grpc":
		rr = grpc.NewResolver(ropts...)
	}

	switch Handler {
	case "rpc":
		log.Infof("Registering API RPC Handler at %s", APIPath)
		rt := regRouter.NewRouter(
			router.WithHandler(arpc.Handler),
			router.WithResolver(rr),
			router.WithRegistry(muregistry.DefaultRegistry),
		)
		rp := arpc.NewHandler(
			ahandler.WithNamespace(Namespace),
			ahandler.WithRouter(rt),
			ahandler.WithClient(srv.Client()),
		)
		r.PathPrefix(APIPath).Handler(rp)
	case "api":
		log.Infof("Registering API Request Handler at %s", APIPath)
		rt := regRouter.NewRouter(
			router.WithHandler(aapi.Handler),
			router.WithResolver(rr),
			router.WithRegistry(muregistry.DefaultRegistry),
		)
		ap := aapi.NewHandler(
			ahandler.WithNamespace(Namespace),
			ahandler.WithRouter(rt),
			ahandler.WithClient(srv.Client()),
		)
		r.PathPrefix(APIPath).Handler(ap)
	case "event":
		log.Infof("Registering API Event Handler at %s", APIPath)
		rt := regRouter.NewRouter(
			router.WithHandler(event.Handler),
			router.WithResolver(rr),
			router.WithRegistry(muregistry.DefaultRegistry),
		)
		ev := event.NewHandler(
			ahandler.WithNamespace(Namespace),
			ahandler.WithRouter(rt),
			ahandler.WithClient(srv.Client()),
		)
		r.PathPrefix(APIPath).Handler(ev)
	case "http":
		log.Infof("Registering API HTTP Handler at %s", ProxyPath)
		rt := regRouter.NewRouter(
			router.WithHandler(ahttp.Handler),
			router.WithResolver(rr),
			router.WithRegistry(muregistry.DefaultRegistry),
		)
		ht := ahttp.NewHandler(
			ahandler.WithNamespace(Namespace),
			ahandler.WithRouter(rt),
			ahandler.WithClient(srv.Client()),
		)
		r.PathPrefix(ProxyPath).Handler(ht)
	case "web":
		log.Infof("Registering API Web Handler at %s", APIPath)
		rt := regRouter.NewRouter(
			router.WithHandler(web.Handler),
			router.WithResolver(rr),
			router.WithRegistry(muregistry.DefaultRegistry),
		)
		w := web.NewHandler(
			ahandler.WithNamespace(Namespace),
			ahandler.WithRouter(rt),
			ahandler.WithClient(srv.Client()),
		)
		r.PathPrefix(APIPath).Handler(w)
	default:
		log.Infof("Registering API Default Handler at %s", APIPath)
		rt := regRouter.NewRouter(
			router.WithResolver(rr),
			router.WithRegistry(muregistry.DefaultRegistry),
		)
		r.PathPrefix(APIPath).Handler(handler.Meta(srv, rt, Namespace))
	}

	// register all the http handler plugins
	for _, p := range plugin.Plugins() {
		if v := p.Handler(); v != nil {
			h = v(h)
		}
	}

	// append the auth wrapper
	h = auth.Wrapper(rr, Namespace)(h)

	// create a new api server with wrappers
	api := httpapi.NewServer(Address)
	// initialise
	api.Init(opts...)
	// register the handler
	api.Handle("/", h)

	// Start API
	if err := api.Start(); err != nil {
		log.Fatal(err)
	}

	// Run server
	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}

	// Stop API
	if err := api.Stop(); err != nil {
		log.Fatal(err)
	}

	return nil
}

module github.com/lack-io/vine

go 1.15

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/bitly/go-simplejson v0.5.0
	github.com/bwmarrin/discordgo v0.22.1
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e
	github.com/cloudflare/cloudflare-go v0.10.2
	github.com/coreos/etcd v3.3.25+incompatible
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/evanphx/json-patch/v5 v5.2.0
	github.com/fasthttp/websocket v1.4.3
	github.com/felixge/httpsnoop v1.0.1
	github.com/forestgiant/sliceutil v0.0.0-20160425183142-94783f95db6c
	github.com/fsnotify/fsnotify v1.4.9
	github.com/fsouza/go-dockerclient v1.7.0
	github.com/go-git/go-git/v5 v5.2.0
	github.com/gofiber/fiber/v2 v2.7.1
	github.com/gogo/protobuf v1.3.2
	github.com/google/uuid v1.2.0
	github.com/hashicorp/hcl v1.0.0
	github.com/hpcloud/tail v1.0.0
	github.com/imdario/mergo v0.3.11
	github.com/jinzhu/inflection v1.0.0
	github.com/jinzhu/now v1.1.1
	github.com/json-iterator/go v1.1.10
	github.com/kr/pretty v0.2.1
	github.com/lack-io/cli v1.2.4
	github.com/lack-io/gscheduler v0.2.0
	github.com/lucas-clemente/quic-go v0.19.3
	github.com/miekg/dns v1.1.35
	github.com/mitchellh/hashstructure v1.1.0
	github.com/nlopes/slack v0.6.0
	github.com/olekukonko/tablewriter v0.0.5
	github.com/oxtoacart/bpool v0.0.0-20190530202638-03653db5a59c
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/rakyll/statik v0.1.7
	github.com/serenize/snaker v0.0.0-20201027110005-a7ad2135616e
	github.com/stretchr/testify v1.7.0
	github.com/technoweenie/multipartstreamer v1.0.1 // indirect
	github.com/valyala/fasthttp v1.23.0
	github.com/xlab/treeprint v1.0.0
	go.uber.org/zap v1.16.0 // indirect
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83
	golang.org/x/net v0.0.0-20210226101413-39120d07d75e
	google.golang.org/grpc v1.35.0
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/telegram-bot-api.v4 v4.6.4
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
)

replace google.golang.org/grpc => google.golang.org/grpc v1.26.0

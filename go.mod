module github.com/cloud-barista/cb-tumblebug

go 1.14

replace (
	github.com/coreos/bbolt => go.etcd.io/bbolt v1.3.3
	github.com/coreos/go-systemd => github.com/coreos/go-systemd/v22 v22.0.0
	google.golang.org/grpc => google.golang.org/grpc v1.26.0
)

require (
	dmitri.shuralyov.com/gpu/mtl v0.0.0-20191203043605-d42048ed14fd // indirect
	github.com/BurntSushi/xgb v0.0.0-20200324125942-20f126ea2843 // indirect
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751
	github.com/armon/consul-api v0.0.0-20180202201655-eb2c6b5be1b6 // indirect
	github.com/beego/beego/v2 v2.0.1
	github.com/bramvdbogaerde/go-scp v0.0.0-20210327204631-70ee53679fc9
	github.com/cloud-barista/cb-log v0.3.1
	github.com/cloud-barista/cb-spider v0.3.9
	github.com/cloud-barista/cb-store v0.3.2
	github.com/cncf/udpa/go v0.0.0-20200327203949-e8cd3a4bb307 // indirect
	github.com/coreos/bbolt v1.3.4 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/envoyproxy/go-control-plane v0.9.5 // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/go-openapi/spec v0.20.3 // indirect
	github.com/go-openapi/swag v0.19.15 // indirect
	github.com/go-resty/resty/v2 v2.6.0
	github.com/go-sql-driver/mysql v1.6.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.2
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/google/uuid v1.2.0
	github.com/gopherjs/gopherjs v0.0.0-20200217142428-fce0ec30dd00 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/grpc-ecosystem/grpc-gateway v1.15.2 // indirect
	github.com/jonboulle/clockwork v0.2.2 // indirect
	github.com/kr/pretty v0.2.0 // indirect
	github.com/labstack/echo v3.3.10+incompatible // indirect
	github.com/labstack/echo/v4 v4.3.0
	github.com/magiconair/properties v1.8.5 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
	github.com/mitchellh/mapstructure v1.4.1 // indirect
	github.com/moby/moby v1.13.1 // indirect
	github.com/opentracing/opentracing-go v1.2.0
	github.com/pelletier/go-toml v1.9.0 // indirect
	github.com/prometheus/client_golang v1.10.0
	github.com/prometheus/common v0.23.0 // indirect
	github.com/rogpeppe/go-internal v1.5.2 // indirect
	github.com/shiena/ansicolor v0.0.0-20200904210342-c7312218db18 // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/smartystreets/assertions v1.1.0 // indirect
	github.com/spf13/afero v1.6.0 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.7.1
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/stretchr/testify v1.7.0
	github.com/swaggo/echo-swagger v1.1.0
	github.com/swaggo/swag v1.7.0
	github.com/tidwall/gjson v1.7.5
	github.com/tidwall/sjson v1.1.6
	github.com/tmc/grpc-websocket-proxy v0.0.0-20200427203606-3cfed13b9966 // indirect
	github.com/uber/jaeger-client-go v2.28.0+incompatible
	github.com/uber/jaeger-lib v2.4.1+incompatible // indirect
	github.com/ugorji/go v1.1.4 // indirect
	github.com/xordataexchange/crypt v0.0.3-0.20170626215501-b2862e3d0a77 // indirect
	github.com/xujiajun/nutsdb v0.6.0 // indirect
	github.com/xwb1989/sqlparser v0.0.0-20180606152119-120387863bf2
	go.etcd.io/bbolt v1.3.5 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	golang.org/x/crypto v0.0.0-20210506145944-38f3c27a63bf
	golang.org/x/image v0.0.0-20200119044424-58c23975cae1 // indirect
	golang.org/x/mobile v0.0.0-20200329125638-4c31acba0007 // indirect
	golang.org/x/net v0.0.0-20210510095157-81045d8b478c // indirect
	golang.org/x/sys v0.0.0-20210507161434-a76c4d0a0096 // indirect
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba // indirect
	google.golang.org/genproto v0.0.0-20210506142907-4a47615972c2 // indirect
	google.golang.org/grpc v1.37.0
	gopkg.in/ini.v1 v1.62.0 // indirect
	gopkg.in/yaml.v2 v2.4.0
	rsc.io/sampler v1.99.99 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)

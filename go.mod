module github.com/cloud-barista/cb-tumblebug

go 1.13

replace (
	github.com/coreos/go-systemd => github.com/coreos/go-systemd/v22 v22.0.0
	github.com/coreos/bbolt => go.etcd.io/bbolt v1.3.3
	google.golang.org/grpc => google.golang.org/grpc v1.26.0
)

require (
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751
	github.com/astaxie/beego v1.12.2
	github.com/bramvdbogaerde/go-scp v0.0.0-20200518191442-5c8efdd1d925
	github.com/cloud-barista/cb-log v0.2.0-cappuccino.0.20201008023843-31002c0a088d
	github.com/cloud-barista/cb-spider v0.2.0-cappuccino.0.20201014061129-f3ef2afbb1b0
	github.com/cloud-barista/cb-store v0.2.0-cappuccino.0.20201014054737-e2310432d256
	github.com/coreos/etcd v3.3.21+incompatible // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/go-openapi/spec v0.19.8 // indirect
	github.com/go-openapi/swag v0.19.9 // indirect
	github.com/go-resty/resty/v2 v2.3.0
	github.com/go-sql-driver/mysql v1.5.0
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.4.2
	github.com/google/uuid v1.1.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.2
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/labstack/echo v3.3.10+incompatible
	github.com/labstack/echo/v4 v4.1.16
	github.com/mailru/easyjson v0.7.1 // indirect
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
	github.com/mitchellh/go-homedir v1.1.0
	github.com/opentracing/opentracing-go v1.2.0
	github.com/prometheus/client_golang v1.7.1
	github.com/shirou/gopsutil v2.20.4+incompatible
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.7.0
	github.com/stretchr/testify v1.5.1
	github.com/swaggo/echo-swagger v1.0.0
	github.com/swaggo/swag v1.6.7
	github.com/uber/jaeger-client-go v2.25.0+incompatible
	github.com/urfave/cli/v2 v2.2.0 // indirect
	github.com/valyala/fasttemplate v1.1.1 // indirect
	github.com/xwb1989/sqlparser v0.0.0-20180606152119-120387863bf2
	go.etcd.io/etcd v3.3.21+incompatible // indirect
	golang.org/x/crypto v0.0.0-20201012173705-84dcc777aaee
	golang.org/x/text v0.3.3 // indirect
	golang.org/x/tools v0.0.0-20200626171337-aa94e735be7f // indirect
	google.golang.org/grpc v1.33.0
	gopkg.in/yaml.v2 v2.3.0
)

module github.com/yob/home-data

go 1.23

require (
	github.com/DataDog/datadog-api-client-go v1.16.0
	github.com/buxtronix/go-daikin v0.0.0-20190717113654-3f7a3f22ebfd
	github.com/dim13/unifi v0.0.0-20210501215740-9c4485c65866
	github.com/google/uuid v1.3.0
	github.com/jaedle/golang-tplink-hs100 v0.4.1
	github.com/pelletier/go-toml v1.9.3
	github.com/tidwall/gjson v1.12.1
	gitlab.com/jtaimisto/bluewalker v0.2.5
	go.yhsif.com/lifxlan v0.3.1
	gopkg.in/mail.v2 v2.3.1
)

require (
	github.com/DataDog/zstd v1.5.0 // indirect
	github.com/golang/glog v1.2.4 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace github.com/buxtronix/go-daikin => github.com/yob/go-daikin v0.0.0-20210501022443-1ff7469ffc3c

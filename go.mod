module github.com/yob/home-data

go 1.15

require (
	cloud.google.com/go v0.86.0
	github.com/DataDog/datadog-api-client-go v1.1.0
	github.com/asaskevich/EventBus v0.0.0-20200907212545-49d423059eef
	github.com/buxtronix/go-daikin v0.0.0-20190717113654-3f7a3f22ebfd
	github.com/dim13/unifi v0.0.0-20210501215740-9c4485c65866
	github.com/golang/glog v0.0.0-20210429001901-424d2337a529 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2
	github.com/google/uuid v1.2.0
	github.com/pelletier/go-toml v1.9.3
	github.com/tidwall/gjson v1.8.1
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/urfave/cli/v2 v2.3.0
	golang.org/x/net v0.0.0-20210614182718-04defd469f4e // indirect
	golang.org/x/sys v0.0.0-20210630005230-0f9fa26af87c // indirect
	google.golang.org/genproto v0.0.0-20210701191553-46259e63a0a9
	google.golang.org/grpc v1.39.0 // indirect
)

replace github.com/buxtronix/go-daikin => github.com/yob/go-daikin v0.0.0-20210501022443-1ff7469ffc3c

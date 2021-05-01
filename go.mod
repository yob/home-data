module github.com/yob/home-data

go 1.15

require (
	cloud.google.com/go v0.81.0
	github.com/asaskevich/EventBus v0.0.0-20200907212545-49d423059eef
	github.com/buxtronix/go-daikin v0.0.0-20190717113654-3f7a3f22ebfd
	github.com/dim13/unifi v0.0.0-20200709055549-52998aa9c807
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2
	github.com/tidwall/gjson v1.7.4
	github.com/urfave/cli/v2 v2.3.0
	golang.org/x/net v0.0.0-20210423184538-5f58ad60dda6 // indirect
	golang.org/x/sys v0.0.0-20210423185535-09eb48e85fd7 // indirect
	google.golang.org/api v0.45.0 // indirect
	google.golang.org/genproto v0.0.0-20210423144448-3a41ef94ed2b
)

replace github.com/buxtronix/go-daikin => github.com/yob/go-daikin v0.0.0-20210501022443-1ff7469ffc3c

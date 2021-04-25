module github.com/yob/home-data

go 1.15

require (
	github.com/asaskevich/EventBus v0.0.0-20200907212545-49d423059eef
	github.com/buxtronix/go-daikin v0.0.0-20190717113654-3f7a3f22ebfd
	github.com/dim13/unifi v0.0.0-20200709055549-52998aa9c807
	github.com/tidwall/gjson v1.7.4
	github.com/urfave/cli/v2 v2.3.0
)

replace github.com/buxtronix/go-daikin => github.com/yob/go-daikin v0.0.0-20210423141610-20faf507e102

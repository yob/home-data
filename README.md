Collecting various data from home and sending it to datadog.

I'm not totally convinced by home automation - I'm nervous about the time
investment required to build a rube goldberg machine that turns on my hall
light when I walk in the door.

However, *monitoring* my house is interesting.

So far the following devices are monitored and charted in datadog:

* 4x indoor ruuvi tags, for temp/humidity 
* 1x outdoor ruuvi tags, for temp/humidity 
* 3x daikin split systems, for on/off state and power consumption
* 1x fronius solar inverter, for solar generation and overall house power draw 
* human presence at home, via phone reachability on the local LAN

Of those, the daikin splits could also be automated to turn on/off under
different conditions. So far I haven't explored that yet.

Here's part of the dashboard I get:

![datadog dashboard](/images/dashboard.png)

## Why not Home Assistant?

I'm a software guy, and this was a good excuse to explore golang. In particular,
I wanted to try the actor pattern using only channels for cross-goroutine
communication.

The result is ~1500 lines of go (so far) that does just what I need. I have to
maintain it and don't get all the home Assistant bells and whistles, but I'm OK
with that for now. I'm learning a lot along the way.

## Will I ever setup some automation?

If I do any automation, it will probably be focused on energy efficiency and
responding to electricity price signals.

My electricity retailer ([amber](https://www.amberelectric.com.au/)) uses
variable pricing based on the wholesale market, and it might be nice to heat
our hot water when prices are low, turn down the AC when prices are high, or
turn on the heating when prices are negative and we can be paid to consume
electricity.

## Cross compiling for the raspberry pi 4

I generally develop this on my intel laptop, and deploy it to a raspberry pi 4
that's on 24/7. Here's the cross compilation commands:

    GOOS=linux GOARCH=arm GOARM=7 go build -o build-arm64/home-data .
    GOOS=linux GOARCH=arm GOARM=7 go build -o build-arm64/jsontohttp ./jsontohttp/


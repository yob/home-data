collecting data from home and sending it to stackdriver

## Cross compiling for the raspberry pi 4

GOOS=linux GOARCH=arm GOARM=7 go build -o build-arm64/home-data .
GOOS=linux GOARCH=arm GOARM=7 go build -o build-arm64/jsontohttp ./jsontohttp/


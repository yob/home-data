package http

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	conf "github.com/yob/home-data/core/config"
	"github.com/yob/home-data/pubsub"
)

const (
	one_hundred_kb       = 102400
	response_timeout_sec = 2
)

type httpServer struct {
	bus        *pubsub.Pubsub
	paths      []string
	pathsMutex sync.RWMutex
}

func Init(bus *pubsub.Pubsub, config *conf.ConfigSection) {
	publish := bus.PublishChannel()

	listenAddr, err := config.GetString("http_listen_address")
	if err != nil {
		debugLog(publish, "http: error reading http_listen_address from config, defaulting to 127.0.0.1")
		listenAddr = "127.0.0.1"
	}

	port, err := config.GetInt64("http_port")
	if err != nil {
		debugLog(publish, "http: error reading http_port from config, defaulting to 8080")
		port = 8080
	}

	if port > 65536 {
		fatalLog(publish, fmt.Sprintf("http: port must be < 65536 (value: %d)", port))
		return
	}

	server := httpServer{
		bus:   bus,
		paths: []string{},
	}

	// listen for adapters registering paths
	go func() {
		server.listenForPaths()
	}()

	http.HandleFunc("/", server.ServeHTTP)

	err = http.ListenAndServe(fmt.Sprintf("%s:%d", listenAddr, port), nil)
	if err != nil {
		fatalLog(publish, fmt.Sprintf("http: unable to start http server (%v)", err))
	}
}

func (server *httpServer) listenForPaths() {
	subRegister, _ := server.bus.Subscribe("http:register-path")
	defer subRegister.Close()

	chPublish := server.bus.PublishChannel()

	for event := range subRegister.Ch {
		server.pathsMutex.Lock()
		debugLog(chPublish, fmt.Sprintf("http: registering path (%s)", event.Value))
		server.paths = append(server.paths, event.Value)
		server.pathsMutex.Unlock()
	}
}

func (server *httpServer) willServePath(path string) bool {
	server.pathsMutex.RLock()
	defer server.pathsMutex.RUnlock()
	for _, p := range server.paths {
		if p == path {
			return true
		}
	}
	return false
}

func (server *httpServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var wg sync.WaitGroup

	// Do we have anything that'll serve this path? If not, bail early
	if !server.willServePath(r.URL.Path) {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	reqUUID, err := uuid.NewRandom()
	if err != nil {
		http.Error(w, fmt.Sprintf("ERR: %v", err), http.StatusInternalServerError)
		return
	}

	defer r.Body.Close()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("ERR: %v", err), http.StatusBadRequest)
		return
	}

	if len(body) > one_hundred_kb {
		http.Error(w, "ERR: Request body must be 100Kb or less", http.StatusBadRequest)
		return
	}

	wg.Add(1)
	sub, _ := server.bus.Subscribe(fmt.Sprintf("http-response:%s", reqUUID.String()))
	defer sub.Close()

	go func() {
		select {
		case event := <-sub.Ch:
			if event.Type != "http-response" {
				http.Error(w, "Unexpected event", 500)
			} else if event.HttpResponse.Status >= 200 && event.HttpResponse.Status <= 299 {
				w.WriteHeader(event.HttpResponse.Status)
				fmt.Fprintf(w, event.HttpResponse.Body)
			} else {
				http.Error(w, event.HttpResponse.Body, event.HttpResponse.Status)
			}
			wg.Done()
		case <-time.After(response_timeout_sec * time.Second):
			http.Error(w, "Timed out waiting for a response to  be generated", http.StatusServiceUnavailable)
			wg.Done()
		}
	}()

	publish := server.bus.PublishChannel()
	publish <- pubsub.PubsubEvent{
		Topic: fmt.Sprintf("http-request:%s", r.URL.Path),
		Data:  pubsub.NewHttpRequestEvent(string(body), reqUUID.String()),
	}

	// don't leave the func until a response has been published back to the bus
	wg.Wait()
}

func debugLog(publish chan pubsub.PubsubEvent, message string) {
	publish <- pubsub.PubsubEvent{
		Topic: "log:new",
		Data:  pubsub.NewKeyValueEvent("DEBUG", message),
	}

}

func fatalLog(publish chan pubsub.PubsubEvent, message string) {
	publish <- pubsub.PubsubEvent{
		Topic: "log:new",
		Data:  pubsub.NewKeyValueEvent("FATAL", message),
	}

}

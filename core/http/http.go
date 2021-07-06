package http

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

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

func Init(bus *pubsub.Pubsub, port int) {
	server := httpServer{
		bus:   bus,
		paths: []string{},
	}

	// listen for adapters registering paths
	go func() {
		server.listenForPaths()
	}()

	http.HandleFunc("/", server.ServeHTTP)

	err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port), nil)
	if err != nil {
		publish := server.bus.PublishChannel()
		fatalLog(publish, fmt.Sprintf("http: unable to start http server (%v)", err))
	}
}

func (server *httpServer) listenForPaths() {
	chRegister := server.bus.Subscribe("http:register-path")
	chPublish := server.bus.PublishChannel()

	for event := range chRegister {
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
	ch_response := server.bus.Subscribe(fmt.Sprintf("http-response:%s", reqUUID.String()))
	go func() {
		// We'll only wait this long for a response to arrive on the bus, then return an error
		timeout := make(chan bool, 1)
		go func() {
			time.Sleep(response_timeout_sec * time.Second)
			timeout <- true
		}()

		select {
		case event := <-ch_response:
			responseCode, err := strconv.Atoi(event.Key)
			if err == nil {
				if responseCode >= 200 && responseCode <= 299 {
					w.WriteHeader(responseCode)
					fmt.Fprintf(w, event.Value)
				} else {
					http.Error(w, event.Value, responseCode)
				}
			} else {
				http.Error(w, fmt.Sprintf("ERR: failed to generate status code (%v)", err), http.StatusInternalServerError)
			}
			wg.Done()
		case <-timeout:
			http.Error(w, "Timed out waiting for a response to  be generated", http.StatusServiceUnavailable)
			wg.Done()
		}
	}()

	// TODO we need a richer way to pass the HTTP request across the bus, including headers
	publish := server.bus.PublishChannel()
	publish <- pubsub.PubsubEvent{
		Topic: fmt.Sprintf("http-request:%s", r.URL.Path),
		Data:  pubsub.KeyValueData{Key: reqUUID.String(), Value: string(body)},
	}

	// don't leave the func until a response has been published back to the bus
	wg.Wait()
}

func debugLog(publish chan pubsub.PubsubEvent, message string) {
	publish <- pubsub.PubsubEvent{
		Topic: "log:new",
		Data:  pubsub.KeyValueData{Key: "DEBUG", Value: message},
	}

}

func fatalLog(publish chan pubsub.PubsubEvent, message string) {
	publish <- pubsub.PubsubEvent{
		Topic: "log:new",
		Data:  pubsub.KeyValueData{Key: "FATAL", Value: message},
	}

}

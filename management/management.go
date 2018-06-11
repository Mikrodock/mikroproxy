package management

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

var server *http.Server
var wgGroup *sync.WaitGroup
var srvMap = make(map[string]*Service)
var srvMutex = &sync.RWMutex{}

func init() {
	r := mux.NewRouter()
	r.HandleFunc("/services/", getServices).Methods("GET")
	r.HandleFunc("/services/", postServices).Methods("POST")
	r.HandleFunc("/services/{stack}/{service}", deleteService).Methods("DELETE")

	server = &http.Server{Addr: ":10512"}
	server.Handler = r
}

func StartManagementServer(quitSignal chan bool, wg *sync.WaitGroup) {

	wgGroup = wg

	wgGroup.Add(1)

	go func(quit chan bool) {
		log.Printf("Starting management server\n")
		server.ListenAndServe()
		timeoutContext, cancel := context.WithTimeout(context.Background(), time.Duration(5*time.Second))
		defer cancel()

		<-quit

		server.Shutdown(timeoutContext)
		log.Printf("Management server quitting")

		srvMutex.RLock()
		for _, srv := range srvMap {
			srv.quitChannel <- true
		}
		srvMutex.RUnlock()

		wgGroup.Done()

	}(quitSignal)

}

func getServices(w http.ResponseWriter, r *http.Request) {
	srvMutex.RLock()
	bytes, err := json.Marshal(srvMap)
	srvMutex.RUnlock()
	if err != nil {
		http.Error(w, err.Error(), 500)
	} else {
		w.Write(bytes)
	}
}

func deleteService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	identifier := vars["stack"] + "/" + vars["service"]

	srvMutex.Lock()
	if srv, ok := srvMap[identifier]; ok {
		srv.quitChannel <- true
		delete(srvMap, identifier)
		w.WriteHeader(200)
	} else {
		http.Error(w, "No service "+identifier, 404)
	}
	srvMutex.Unlock()
}

func postServices(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 500)
	}

	var srvCreationRequest ServiceCreationRequest

	err = json.Unmarshal(body, &srvCreationRequest)
	if err != nil {
		http.Error(w, err.Error(), 500)
	}

	service, err := CreateService(srvCreationRequest.ServiceName, srvCreationRequest.StackName, srvCreationRequest.PublicPort, srvCreationRequest.InternalPort, wgGroup)
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
	srvMutex.Lock()
	srvMap[srvCreationRequest.StackName+"/"+srvCreationRequest.ServiceName] = service
	srvMutex.Unlock()
}

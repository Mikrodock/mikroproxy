package management

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/miekg/dns"
)

var externalBoundsPort = make(map[int]*Service, 0)
var dnsServer string

type Service struct {
	quitChannel chan bool
	identifier  string
	listenPort  int
	lookup      string
	preparedURI string
}

type ServiceCreationRequest struct {
	ServiceName  string `json:"service_name"`
	StackName    string `json:"stack_name"`
	PublicPort   int    `json:"public_port"`
	InternalPort int    `json:"internal_port"`
}

func (s *Service) doDNS() string {
	c := dns.Client{}
	m := dns.Msg{}
	m.SetQuestion(s.lookup+".", dns.TypeA)
	r, _, err := c.Exchange(&m, dnsServer+":53")
	retries := 0
	done := false
	for retries < 3 && !done {
		r, _, err = c.Exchange(&m, dnsServer+":53")
		if err != nil {
			retries++
		} else {
			done = true
		}
	}

	if err != nil {
		log.Fatal(err)
	}

	if len(r.Answer) == 0 {
		log.Fatal("No results")
	}
	Arecord := r.Answer[0].(*dns.A)
	return Arecord.A.String()
}

func (s *Service) Start(wg *sync.WaitGroup) {
	server := &http.Server{Addr: ":" + strconv.Itoa(s.listenPort)}
	server.Handler = s
	wg.Add(1)
	go func() {

		log.Printf("%s : starting server\n", s.identifier)

		if err := server.ListenAndServe(); err != nil {
			// cannot panic, because this probably is an intentional close
			log.Printf("Httpserver: ListenAndServe() error: %s", err)
			wg.Done()
		}

		<-s.quitChannel

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)

		log.Printf("%s : server stopped\n", s.identifier)

		wg.Done()
	}()
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	ip := s.doDNS()

	uri := fmt.Sprintf(s.preparedURI, ip) + r.RequestURI

	fmt.Println(r.Method + ": " + uri)

	if r.Method == "POST" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte(fmt.Sprintln("Cannot read body", err.Error())))
		}
		fmt.Printf("Body: %v\n", string(body))
	}
	rr, err := http.NewRequest(r.Method, uri, r.Body)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintln("Cannot create request", err.Error())))
	}
	copyHeader(r.Header, &rr.Header)

	// Create a client and query the target
	var transport http.Transport
	resp, err := transport.RoundTrip(rr)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintln("Cannot join host", err.Error())))
	} else {
		fmt.Printf("Resp-Headers: %v\n", resp.Header)

		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		fatal(err)

		dH := w.Header()
		copyHeader(resp.Header, &dH)
		dH.Add("Requested-Host", rr.Host)

		w.Write(body)
	}

}

func CreateService(serviceName, stackName string, publicPort, internalPort int, wg *sync.WaitGroup) (*Service, error) {
	if service, alreadyBound := externalBoundsPort[publicPort]; alreadyBound {
		return nil, errors.New(fmt.Sprintln("Port", publicPort, "is already bound by service", service.identifier))
	}

	schema := "http"
	if internalPort == 443 {
		schema = "https"
	}

	target := schema + "://" + serviceName + "." + stackName + ".mikrodock:" + strconv.Itoa(internalPort)

	service := &Service{
		identifier:  target,
		quitChannel: make(chan bool),
		listenPort:  publicPort,
		lookup:      serviceName + "." + stackName + ".mikrodock",
		preparedURI: schema + "://%s:" + strconv.Itoa(internalPort),
	}

	fmt.Println("Starting service")

	service.Start(wg)

	return service, nil
}

func fatal(err error) {
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

func copyHeader(source http.Header, dest *http.Header) {
	for n, v := range source {
		for _, vv := range v {
			dest.Add(n, vv)
		}
	}
}

func SetDNS(dns string) {
	dnsServer = dns
}

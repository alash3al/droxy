package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

import (
	"github.com/fsouza/go-dockerclient"
	"github.com/vulcand/oxy/forward"
	"github.com/vulcand/oxy/roundrobin"
	"golang.org/x/crypto/acme/autocert"
)

var (
	// the current version of droxy
	DROXY_VERSION = "DROXY/2.0"

	// the environment var that handles the hosts mappings [hostname:port,hostname2:port]
	DROXY_HOST = "DROXY_HOST"

	// the environment var that handles the hostnames that will use Let'sEncrypt
	DROXY_LETSENCRYPT = "DROXY_LETSENCRYPT"

	// mapping between containerID => service [backends]
	SERVICES = map[string]*Service{}

	// CMD flags
	HTTP_ADDR  = flag.String("http", ":80", "the default http port to bind droxy")
	HTTPS_ADDR = flag.String("https", ":443", "the default https port to bind droxy")
	HTTPS_DIR  = flag.String("certs-dir", "~/.droxy-certs", "the default directory to cache letsencrypt certs")
)

// a service contains the container, host => port and letsencrypt hosts
type Service struct {
	Container   *docker.Container
	Mappings    map[string]*url.URL
	LetsEncrypt []string
}

// let's play :)
func main() {
	flag.Parse()
	loadServices()
	go func() { watchServices() }()
	go func() { log.Fatal(listenAndServeAutoCert()) }()
	log.Fatal(http.ListenAndServe(*HTTP_ADDR, http.HandlerFunc(handler)))
}

// this is our autossl [based on let's encrypt] handler
func listenAndServeAutoCert() error {
	m := autocert.Manager{
		Prompt: autocert.AcceptTOS,
		HostPolicy: func(_ context.Context, hostname string) error {
			for _, service := range SERVICES {
				for _, h := range service.LetsEncrypt {
					if regexp.MustCompile(`^` + strings.Replace(h, "*", "(.*)", -1) + `$`).MatchString(hostname) {
						return nil
					}
				}
			}
			return errors.New("The requested hostname (" + hostname + ") is undefined")
		},
		Cache: autocert.DirCache(*HTTPS_DIR),
	}
	s := &http.Server{
		Addr:      *HTTPS_ADDR,
		TLSConfig: &tls.Config{GetCertificate: m.GetCertificate},
	}
	return s.ListenAndServeTLS("", "")
}

// fetch all containers and fetch droxy services from them
// then cache the result into our global cache
func loadServices() {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	ps, err := client.ListContainers(docker.ListContainersOptions{All: false})
	if err != nil {
		log.Fatal(err)
	}
	for _, c := range ps {
		if c.State == "running" {
			container, err := client.InspectContainer(c.ID)
			if err != nil {
				continue
			}
			addService(container)
		}
	}
}

// this function watches docker server for events
// and then process them .
func watchServices() {
	// create a new docker client connection from the environment
	client, err := docker.NewClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	// here we listen for changes
	events := make(chan *docker.APIEvents)
	err = client.AddEventListener(events)
	if err != nil {
		log.Fatal(err)
	}

	// close the events channel
	defer client.RemoveEventListener(events)

	// here we read the events and then process them
	for event := range events {
		if event.Type == "container" {
			container, err := client.InspectContainer(event.Actor.ID)
			if err != nil {
				continue
			}
			if event.Action == "start" {
				addService(container)
			} else if event.Action == "stop" {
				removeService(container.ID)
			}
		}
	}
}

// adding a service to our cache
func addService(container *docker.Container) {
	// convert the env vars to query-values to make the code simple
	env, err := url.ParseQuery(strings.Join(container.Config.Env, "&"))
	if err != nil {
		return
	}

	// a droxy service ?
	if env.Get(DROXY_HOST) == "" {
		return
	}

	// fetch the ports mapping of this container
	// if there is no ports, don't cotinue
	servicePorts := container.NetworkSettings.PortMappingAPI()
	if len(servicePorts) < 1 {
		return
	}

	// the default private port is "80",
	// else, if the ports are just [one port], then use that private port as the default
	defaultPort := "80"
	if len(servicePorts) == 1 {
		defaultPort = strconv.Itoa(int(servicePorts[0].PrivatePort))
	}

	// here we hold our hostname => urls
	mappings := map[string]*url.URL{}

	// loop over the privded hosts in the DROXY_HOST
	// fix each host's scheme
	// set the port of each hostname, [default is "$defaultPort"]
	// then loop over each port in the servicePorts to find the public port of each hostname->[port]
	// and finally add each processed url to the mappings
	for _, host := range strings.Split(env.Get(DROXY_HOST), ",") {
		if !strings.Contains(host, "://") {
			host = "http://" + host
		}
		parsedURL, _ := url.Parse(host)
		selectedPort := parsedURL.Port()
		if selectedPort == "" {
			selectedPort = defaultPort
		}
		for _, servicePort := range servicePorts {
			if selectedPort == strconv.Itoa(int(servicePort.PrivatePort)) {
				mappings[parsedURL.Hostname()], _ = url.Parse(parsedURL.Scheme + "://" + servicePort.IP + ":" + strconv.Itoa(int(servicePort.PublicPort)))
			}
		}
	}

	// register the service in the global cache
	SERVICES[container.ID] = &Service{
		Container:   container,
		Mappings:    mappings,
		LetsEncrypt: strings.Split(env.Get(DROXY_LETSENCRYPT), ","),
	}
}

// removing a service [by container id] from the cache
func removeService(container_id string) {
	delete(SERVICES, container_id)
}

// find a service by hostname and return its backends
func resolveService(hostname string) []*url.URL {
	result := []*url.URL{}
	for _, service := range SERVICES {
		if v, ok := service.Mappings[hostname]; ok {
			result = append(result, v)
			continue
		}
		for k, v := range service.Mappings {
			if regexp.MustCompile(`^` + strings.Replace(k, "*", "(.*)", -1) + `$`).MatchString(hostname) {
				result = append(result, v)
			}
		}
	}
	return result
}

// the http handler
func handler(res http.ResponseWriter, req *http.Request) {
	backends := resolveService(req.Host)
	res.Header().Set("Server", "droxy/2.0")
	if len(backends) < 1 {
		http.NotFound(res, req)
		log.Println("Resolver: service (" + req.Host + ") not found")
		return
	}
	fwd, _ := forward.New()
	lb, _ := roundrobin.New(fwd)
	for _, backend := range backends {
		lb.UpsertServer(backend)
	}
	lb.ServeHTTP(res, req)
}

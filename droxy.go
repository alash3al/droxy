// droxy "Docker Proxy" a standalone docker http proxy,
// created by Mohammed Al Ashaal, and released under MIT License .
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
)

import "github.com/fsouza/go-dockerclient"

var (
	listenAddr   = flag.String("addr", ":80", "the listen address")
	dockerSocket = flag.String("docker", "unix:///var/run/docker.sock", "the docker endpoint")
)

func init() {
	flag.Parse()
}

func main() {
	client, err := docker.NewClient(*dockerSocket)
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(http.ListenAndServe(*listenAddr, http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()
		apps, err := client.ListContainers(docker.ListContainersOptions{})
		if err != nil {
			http.Error(res, http.StatusText(503), 503)
			return
		}
		ok, addr := findApp(req.Host, apps)
		if !ok {
			log.Fatal(ok, addr)
			http.Error(res, http.StatusText(503), 503)
			return
		}
		backend, err := http.NewRequest(req.Method, addr, req.Body)
		if err != nil {
			log.Fatal(err)
			http.Error(res, http.StatusText(503), 503)
			return
		}
		client := &http.Client{
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return errors.New("STOP")
			},
		}
		backend.Header = req.Header
		uip, uport, _ := net.SplitHostPort(req.RemoteAddr)
		backend.Host = req.Host
		backend.Header.Set("Host", req.Host)
		backend.Header.Set("X-Real-IP", uip)
		backend.Header.Set("X-Remote-IP", uip)
		backend.Header.Set("X-Remote-Port", uport)
		backend.Header.Set("X-Forwarded-For", uip)
		backend.Header.Set("X-Forwarded-Proto", "https")
		backend.Header.Set("X-Forwarded-Host", req.Host)
		result, err := client.Do(backend)
		if err != nil {
			log.Fatal(err)
			http.Error(res, http.StatusText(503), 503)
			return
		}
		defer result.Body.Close()
		for k, vals := range result.Header {
			for _, v := range vals {
				res.Header().Add(k, v)
			}
		}
		res.WriteHeader(result.StatusCode)
		io.Copy(res, result.Body)
	})))
}

// This function will iterate over available containers
// and get the container which its name equals or maybe a suffix in
// the requested hostname "", also it will search for the target public
// port that maps to "private 80 port", or the private port defined in the
// requested "Host" header .
func findApp(host string, apps []docker.APIContainers) (bool, string) {
	hostNameAndPort := strings.SplitN(host, ":", 2)
	if len(hostNameAndPort) == 1 {
		hostNameAndPort = append(hostNameAndPort, "")
	}
	host_name, host_port := hostNameAndPort[0], hostNameAndPort[1]
	for _, app := range apps {
		for _, name := range app.Names {
			name = strings.Trim(name, "/")
			if host_name == name || strings.HasSuffix(host_name, "."+name) {
				for _, port := range app.Ports {
					if port.PrivatePort == 80 || fmt.Sprintf("%d", port.PrivatePort) == host_port {
						return true, fmt.Sprintf("http://0.0.0.0:%d", port.PublicPort)
					}
				}
			}
		}
	}
	return false, ""
}

# Droxy
a transparent standalone http reverse proxy for docker containers .

# Installation
```bash
$ go get github.com/alash3al/droxy
$ go install github.com/alash3al/droxy
```

# Usage
- create a simple docker app that listens on any port i.e `-p 8001:80`
- give it a name `--name=container1.domain.com`
- now after mapping the domain name of `container1.domain.com` to the main server ip address, you can call it to see its response .

# Help
```bash
alash3al@laptop:~$ droxy --help
Usage of droxy:
  -addr string
    	the listen-address to serve the request (default ":80")
  -docker string
    	the docker socket path (default "unix:///var/run/docker.sock")

```

# Examples
> droxy will first search for the public container port that maps to the private "80" port  
```bash
$ droxy &
$ docker run --name example1.localhost -d -p 8080:80 -v /some/content:/usr/share/nginx/html:ro -d nginx
$ curl localhost -H "Host: example1.localhost"
```

> or it will allows you to change that policy to search for the port that maps to the private {Port In `Host` Header}  
```bash
$ droxy &
$ docker run --name example2.localhost -d -p 8081:81 -v /some/content:/usr/share/nginx/html:ro -d nginx
$ curl localhost -H "Host: example2.localhost:81"
```
** any error will break the request and returns 503

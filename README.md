# Droxy
A transparent standalone http reverse proxy for docker containers .

# Download
- [Goto releases](https://github.com/alash3al/droxy/releases/tag/v2.0).
- Then open your `terminal` or `CMD` .

# Usage
To run it, just run the downloaded binary as it !
```bash
# type ./droxy for linux or ./droxy.exe for windows
$ ./droxy
```

Then You need to run any container(s), for examples, we will use the default nginx container.
```bash
docker run --name service1 -v /var/www/:/usr/share/nginx/html:ro -d -p 8081:80 -e DROXY_HOST=service1.mysite.com -e DROXY_LETSENCRYPT=service1.mysite.com nginx
```
now open `service1.mysite.com` which maps to your server's ip, you will see the nginx service .


What is `DROXY_HOST` and `DROXY_LETSENCRYPT` ?
- `DROXY_HOST` tells droxy to route all requests that match `DROXY_HOST` to this container . 
- `DROXY_LETSENCRYPT` tells droxy to enable auto ssl based on Let'sEncrypt for this container and that hostname .

# Features
- No Dependencies, just a small single binary ! .
- Watches docker in realtime and add/remove containers from our own internal service discovery .
- Automatically generate and renew SSL Certs for created containers .
- Single and Multiple Hosts allowed for both `DROXY_HOST` and `DROXY_LETSENCRYPT`
- You can specifiy whether to use `http` or `https` when connecting with the backend `DROXY_HOST=host1.com,https://host2.com`.
- You can choose the backend port (docker private port) to be used for each hostname `DROXY_HOST=host1.com,host2.com:8080`
- You can run multiple containers with the same hostname and droxy will use `roundrobin` to distribute the traffic between them .
- You can use wildcards with hostnames for both `DROXY_HOST` and `DROXY-LETSENCRYPT`.
- It caches Let'sEncrypt certs in the current working directory under `./droxy-certs/`, you can change when starting as following `./droxy --certs-dir=/path/to/custom/dir`.
- You can change the default listening ports for both `80` and `443` `./droxy --http=:8080 --https=:44303`

# Installing Source
```bash
$ go get -u github.com/alash3al/droxy
$ go install github.com/alash3al/droxy
$ droxy --help
```

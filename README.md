# Erebus

Erebus is a little HTTP reverse proxy.

Erebus is the successor to [rrproxy](https://github.com/cespare/rrproxy) and
[ez-nginx-proxy](https://github.com/cespare/ez-nginx-proxy) and fills similar use cases, but is more flexible.

Requests are matched against rules, and routed according to the first matching rule.

More info forthcoming.

## Quick example

conf.json:

``` json
[
  {"from": {"path": "/foo"},
   "to":   {"addr": "localhost:8100"}},
  {"from": {"host": "example.com"},
   "to":   {"addr": "localhost:8101"}}
]
```

Suppose you have the entry `127.0.0.1 example.com` in your `/etc/hosts`, and that erebus is running with the
above configuration on port 80. Then:

* Requests to `localhost/foo` would be proxied to `localhost:8100`
* Requests to `localhost/bar` would get an HTTP 502 error (bad gateway)
* Requests to `example.com` would be proxied to `localhost:8101`

## To Do

* `from` filtering:
  - `remote-addr` A particular remote address
  - `header` Some header and optionally, a particular value
  - `querystring` Some querystring parameter and optionally, a particular value
* `to` modifications:
  - `addr` is required
  - `path-prefix`
  - `header` Set some header to some value
  - `querystring` add some querystring parameters
* (Configurable) timeouts

# Erebus

Erebus is a little HTTP reverse proxy.

This is the successor to [rrproxy](https://github.com/cespare/rrproxy) and
[ez-nginx-proxy](https://github.com/cespare/ez-nginx-proxy).

More info forthcoming.

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

# Erebus

Erebus is a little HTTP reverse proxy.

More info forthcoming.

## To Do

* `from` filtering:
  - `remote-addr` A particular remote address
  - `header` Some header and optionally, a particular value
  - `path` Some request path, possibly with wildcards or regexes
  - `querystring` Some querystring parameter and optionally, a particular value
* `to` modifications:
  - `addr` is required
  - `path-prefix`
  - `header` Set some header to some value
  - `querystring` add some querystring parameters
* (Configurable) timeouts

package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

type Conf struct {
	From *FromConf
	To   *ToConf
}

type FromConf struct {
	Host string
}

// Matches determines whether an HTTP request matches this configuration.
func (c *FromConf) Matches(r *http.Request) bool {
	if c.Host != "" && c.Host != r.Host {
		return false
	}
	return true
}

type ToConf struct {
	Addr string
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

// CreateRequest synthesizes a new http.Request by applying this ToConf's configuration to an inbound request.
// NOTE: Most of this logic was copied from net/http/httputil.ReverseProxy.
func (c *ToConf) CreateRequest(r *http.Request) *http.Request {
	out := &http.Request{}
	*out = *r // Note this shallow copies maps

	// Apply configuration
	if c.Addr != "" {
		out.URL.Host = c.Addr
	}

	if r.TLS == nil {
		out.URL.Scheme = "http"
	} else {
		out.URL.Scheme = "https"
	}

	// Change other settings suitable for reverse proxies
	out.Proto = "HTTP/1.1"
	out.ProtoMajor = 1
	out.ProtoMinor = 1
	out.Close = false

	// Remove hop-by-hop headers to the backend. Especially important is "Connection" because we want a
	// persistent connection, regardless of what the client sent to us. This is modifying the same underlying
	// map from r (shallow copied above) so we only copy it if necessary.
	copiedHeaders := false
	for _, h := range hopHeaders {
		if out.Header.Get(h) != "" {
			if !copiedHeaders {
				out.Header = make(http.Header)
				copyHeader(out.Header, r.Header)
				copiedHeaders = true
			}
			out.Header.Del(h)
		}
	}

	if clientIP, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		// If we aren't the first proxy retain prior X-Forwarded-For information as a comma+space separated list
		// and fold multiple headers into one.
		if prior, ok := out.Header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		out.Header.Set("X-Forwarded-For", clientIP)
	}

	return out
}

type Proxy struct {
	Rules     []*Conf
	Transport http.RoundTripper
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, rule := range p.Rules {
		if rule.From.Matches(r) {
			out := rule.To.CreateRequest(r)
			resp, err := p.Transport.RoundTrip(out)
			if err != nil {
				log.Printf("erebus proxy error: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			defer resp.Body.Close()

			copyHeader(w.Header(), resp.Header)
			w.WriteHeader(resp.StatusCode)
			// TODO: There might be scenarios in which we should implement periodic flushing here
			io.Copy(w, resp.Body)
			return
		}
	}
	http.Error(w, "No matching rule.", http.StatusBadGateway)
}

var (
	listenAddr = flag.String("listenaddr", "localhost:3111", "The address on which erebus should listen")
	configFile = flag.String("conf", "conf.json", "The configuration file to use")
	proxy      *Proxy
)

func init() {
	flag.Parse()
	f, err := os.Open(*configFile)
	if err != nil {
		log.Fatal(err)
	}
	decoder := json.NewDecoder(f)
	rules := []*Conf{}
	if err := decoder.Decode(&rules); err != nil {
		log.Fatalf("Error reading configuration file %s: %s", *configFile, err)
	}
	if len(rules) < 1 {
		log.Fatal("Error with configuration file %s: must include at least one rule.", *configFile)
	}
	proxy = &Proxy{
		Rules:     rules,
		Transport: http.DefaultTransport,
	}
}

func main() {
	log.Println("Now listening on ", *listenAddr)
	log.Fatal(http.ListenAndServe(*listenAddr, proxy))
}

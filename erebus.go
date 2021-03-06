package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type Conf struct {
	From *FromConf
	To   *ToConf
}

func (c *Conf) validate() error {
	if c.From.PathRegex != "" {
		var err error
		c.From.regex, err = regexp.Compile(c.From.PathRegex)
		if err != nil {
			return err
		}
	}
	return nil
}

type FromConf struct {
	Host       string
	Path       string
	PathPrefix string
	PathRegex  string
	regex      *regexp.Regexp
}

// Matches determines whether an HTTP request matches this configuration.
func (c *FromConf) Matches(r *http.Request) bool {
	switch {
	case c.Host != "" && c.Host != r.Host:
		return false
	case c.Path != "" && c.Path != r.URL.Path:
		return false
	case c.PathPrefix != "" && !strings.HasPrefix(r.URL.Path, c.PathPrefix):
		return false
	case c.regex != nil && !c.regex.MatchString(r.URL.Path):
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

// NewProxyFromRules takes a raw JSON configuration and constructs a Proxy from it. It may return an error if
// the rules are malformed or invalid.
func NewProxyFromRules(jsonText []byte) (*Proxy, error) {
	rules := []*Conf{}
	if err := json.Unmarshal(jsonText, &rules); err != nil {
		return nil, err
	}
	if len(rules) < 1 {
		return nil, fmt.Errorf("configuration must include at least one rule.")
	}
	for _, conf := range rules {
		if err := conf.validate(); err != nil {
			return nil, fmt.Errorf("error with configuration: %s", err)
		}
	}
	proxy := &Proxy{
		Rules:     rules,
		Transport: http.DefaultTransport,
	}
	return proxy, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fromLog := Csprintf("[%s] #blue{%s} %s", r.Host, r.Method, r.URL)
	delay := time.Duration(0)
	toLog := ""
	defer func() { LogCprintf("%s #blue{→}  %s", fromLog, toLog) }()

	for _, rule := range p.Rules {
		if rule.From.Matches(r) {
			out := rule.To.CreateRequest(r)

			before := time.Now()
			resp, err := p.Transport.RoundTrip(out)
			delay = time.Since(before)

			if err != nil {
				msg := fmt.Sprintf("backend error: %s", err)
				toLog = Csprintf("%s #red{%s}", rule.To.Addr, msg)
				log.Print(msg)
				http.Error(w, msg, http.StatusInternalServerError)
				return
			}
			defer resp.Body.Close()

			copyHeader(w.Header(), resp.Header)
			w.WriteHeader(resp.StatusCode)
			status := Csprintf("#red{%d}", resp.StatusCode)
			if resp.StatusCode == http.StatusOK {
				status = Csprintf("#green{%d}", resp.StatusCode)
			}
			toLog = Csprintf("%s %s #blue{%.3fs}", rule.To.Addr, status, delay.Seconds())
			// TODO: There might be scenarios in which we should implement periodic flushing here
			io.Copy(w, resp.Body)
			return
		}
	}
	toLog = Csprintf("#red{No matching rule.}")
	http.Error(w, "No matching rule.", http.StatusBadGateway)
}

var (
	listenAddr = flag.String("listenaddr", "localhost:3111", "The address on which erebus should listen")
	configFile = flag.String("conf", "conf.json", "The configuration file to use")
	verbose    = flag.Bool("verbose", false, "Log each request")
	proxy      *Proxy
)

func init() {
	flag.Parse()
	contents, err := ioutil.ReadFile(*configFile)
	if err == nil {
		proxy, err = NewProxyFromRules(contents)
	}
	if err != nil {
		log.Fatalf("Error with configuration %s: %s", *configFile, err)
	}
}

func main() {
	log.Println("Now listening on", *listenAddr)
	log.Fatal(http.ListenAndServe(*listenAddr, proxy))
}

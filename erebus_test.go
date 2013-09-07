package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

var (
	mut sync.Mutex // protects id
	id  = 0
)

func nextId() string {
	mut.Lock()
	defer mut.Unlock()
	id++
	return strconv.Itoa(id)
}

type TestBackend struct {
	*httptest.Server
	LastReceivedRequests string
}

func (b *TestBackend) handle(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("_id")
	b.LastReceivedRequests = id
}

func NewTestBackend() *TestBackend {
	backend := &TestBackend{}
	backend.Server = httptest.NewServer(http.HandlerFunc(backend.handle))
	return backend
}

type TestRequest struct {
	// This describes what the request is testing
	Description string

	// These may be set in TestCases
	Method      string
	Path        string
	QueryParams map[string]string
	Host        string

	// If Status is 0, then it's expected to be a 200 and the appropriate Backend should have received the
	// request. Otherwise, the response should have error code Status. Backends are indexed from 1.
	Status  int
	Backend int
}

type TestCase struct {
	Rules    string
	Requests []*TestRequest
}

var testCases = []TestCase{
	{`[{"from": {"host": "foo.com"},
	    "to":   {"addr": "{{backend1}}"}},
	   {"from": {"host": "bar.com"},
	    "to":   {"addr": "{{backend2}}"}}]`,
		[]*TestRequest{
			{
				Description: "a simple request should be sent to the correct backend for its HOST (1 of 2)",
				Host:        "foo.com",
				Backend:     1,
			},
			{
				Description: "a simple request should be sent to the correct backend for its HOST (2 of 2)",
				Host:        "bar.com",
				Backend:     2,
			},
			{
				Description: "a simple request should get an HTTP 502 if there is no matching backend",
				Host:        "baz.com",
				Status:      http.StatusBadGateway,
			},
		},
	},

	{`[{"from": {"path": "/foo/bar"},
	    "to":   {"addr": "{{backend1}}"}},
	   {"from": {"pathprefix": "/foo/"},
	    "to":   {"addr": "{{backend2}}"}},
	   {"from": {"pathregex": "foo"},
	    "to":   {"addr": "{{backend3}}"}}]`,
		[]*TestRequest{
			{
				Description: "a path rule must match exactly",
				Path:        "/foo/bar",
				Backend:     1,
			},
			{
				Description: "a pathprefix will match if the request path is a prefix of the rule",
				Path:        "/foo/baz",
				Backend:     2,
			},
			{
				Description: "a pathregex is interpreted as a regular expression to match the path",
				Path:        "/a/b/foo/c",
				Backend:     3,
			},
		},
	},
}

func TestCases(t *testing.T) {
	for _, testCase := range testCases {
		maxBackend := 0
		for _, req := range testCase.Requests {
			if req.Backend > maxBackend {
				maxBackend = req.Backend
			}
		}

		config := testCase.Rules
		backends := make([]*TestBackend, maxBackend)
		for i := range backends {
			backends[i] = NewTestBackend()
			defer backends[i].Close()
			url := strings.TrimPrefix(backends[i].URL, "http://")
			config = strings.NewReplacer(fmt.Sprintf("{{backend%d}}", i+1), url).Replace(config)
		}
		proxy, err := NewProxyFromRules([]byte(config))
		if err != nil {
			t.Fatal(err)
		}
		server := httptest.NewServer(proxy)
		defer server.Close()

		for _, req := range testCase.Requests {
			method := req.Method
			if method == "" {
				method = "GET"
			}
			id := nextId()
			url := fmt.Sprintf("%s%s?_id=%s", server.URL, req.Path, id)
			for k, v := range req.QueryParams {
				url += fmt.Sprintf("&%s=%s", k, v)
			}
			request, err := http.NewRequest(method, url, nil)
			if err != nil {
				log.Fatal(err)
			}
			if req.Host != "" {
				request.Host = req.Host
			}
			resp, err := http.DefaultClient.Do(request)
			if err != nil {
				log.Fatal(err)
			}
			defer resp.Body.Close()

			if req.Status == 0 {
				if backends[req.Backend-1].LastReceivedRequests != id {
					log.Fatalf("Error for test request '%s': appropriate backend did not receive request.",
						req.Description)
				}
			} else {
				if resp.StatusCode != req.Status {
					log.Fatalf("Error for test request '%s': expected status %d but got %d", req.Description,
						req.Status, resp.StatusCode)
				}
			}
		}

		req, err := http.NewRequest("GET", fmt.Sprintf("%s?_id=%s", server.URL, nextId()), nil)
		if err != nil {
			log.Fatal(err)
		}
		req.Host = "example2.com"
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()
	}
}

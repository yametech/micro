// Copyright 2020 Asim Aslam
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Original source: github.com/micro/go-micro/v3/api/handler/web.go

// Package web contains the web handler including websocket support
package web

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/micro/micro/v3/service/api"
	"github.com/micro/micro/v3/service/api/server/handler"
	"github.com/micro/micro/v3/service/registry"
)

const (
	Handler = "web"
)

type webHandler struct {
	opts handler.Options
	s    *api.Service
}

func (wh *webHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	service, err := wh.getService(r)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	if len(service) == 0 {
		w.WriteHeader(404)
		return
	}

	rp, err := url.Parse(service)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	if isWebSocket(r) {
		wh.serveWebSocket(rp.Host, w, r)
		return
	}

	httputil.NewSingleHostReverseProxy(rp).ServeHTTP(w, r)
}

// getService returns the service for this request from the selector
func (wh *webHandler) getService(r *http.Request) (string, error) {
	var service *api.Service

	if wh.s != nil {
		// we were given the service
		service = wh.s
	} else if wh.opts.Router != nil {
		// try get service from router
		s, err := wh.opts.Router.Route(r)
		if err != nil {
			return "", err
		}
		service = s
	} else {
		// we have no way of routing the request
		return "", errors.New("no route found")
	}

	// get the nodes
	var nodes []*registry.Node
	for _, srv := range service.Services {
		nodes = append(nodes, srv.Nodes...)
	}
	if len(nodes) == 0 {
		return "", errors.New("no route found")
	}

	// select a random node
	node := nodes[rand.Int()%len(nodes)]

	return fmt.Sprintf("http://%s", node.Address), nil
}

// serveWebSocket used to serve a web socket proxied connection
func (wh *webHandler) serveWebSocket(host string, w http.ResponseWriter, r *http.Request) {
	req := new(http.Request)
	*req = *r

	if len(host) == 0 {
		http.Error(w, "invalid host", 500)
		return
	}

	// set x-forward-for
	if clientIP, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		if ips, ok := req.Header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(ips, ", ") + ", " + clientIP
		}
		req.Header.Set("X-Forwarded-For", clientIP)
	}

	// connect to the backend host
	conn, err := net.Dial("tcp", host)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// hijack the connection
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "failed to connect", 500)
		return
	}

	nc, _, err := hj.Hijack()
	if err != nil {
		return
	}

	defer nc.Close()
	defer conn.Close()

	if err = req.Write(conn); err != nil {
		return
	}

	errCh := make(chan error, 2)

	cp := func(dst io.Writer, src io.Reader) {
		_, err := io.Copy(dst, src)
		errCh <- err
	}

	go cp(conn, nc)
	go cp(nc, conn)

	<-errCh
}

func isWebSocket(r *http.Request) bool {
	contains := func(key, val string) bool {
		vv := strings.Split(r.Header.Get(key), ",")
		for _, v := range vv {
			if val == strings.ToLower(strings.TrimSpace(v)) {
				return true
			}
		}
		return false
	}

	if contains("Connection", "upgrade") && contains("Upgrade", "websocket") {
		return true
	}

	return false
}

func (wh *webHandler) String() string {
	return "web"
}

func NewHandler(opts ...handler.Option) handler.Handler {
	return &webHandler{
		opts: handler.NewOptions(opts...),
	}
}

func WithService(s *api.Service, opts ...handler.Option) handler.Handler {
	options := handler.NewOptions(opts...)

	return &webHandler{
		opts: options,
		s:    s,
	}
}

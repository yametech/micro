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
// Original source: github.com/micro/go-micro/v3/api/router/router_test.go
package router_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/micro/micro/v3/service/api/server/handler"
	"github.com/micro/micro/v3/service/api/server/handler/rpc"
	"github.com/micro/micro/v3/service/api/server/router"
	rregistry "github.com/micro/micro/v3/service/api/server/router/registry"
	"github.com/micro/micro/v3/service/client"
	gcli "github.com/micro/micro/v3/service/client/grpc"
	rmemory "github.com/micro/micro/v3/service/registry/memory"
	rt "github.com/micro/micro/v3/service/router"
	regRouter "github.com/micro/micro/v3/service/router/registry"
	"github.com/micro/micro/v3/service/server"
	gsrv "github.com/micro/micro/v3/service/server/grpc"
	pb "github.com/micro/micro/v3/service/server/grpc/proto"
)

// server is used to implement helloworld.GreeterServer.
type testServer struct {
	msgCount int
}

// TestHello implements helloworld.GreeterServer
func (s *testServer) Call(ctx context.Context, req *pb.Request, rsp *pb.Response) error {
	rsp.Msg = "Hello " + req.Uuid
	return nil
}

// TestHello implements helloworld.GreeterServer
func (s *testServer) CallPcre(ctx context.Context, req *pb.Request, rsp *pb.Response) error {
	rsp.Msg = "Hello " + req.Uuid
	return nil
}

// TestHello implements helloworld.GreeterServer
func (s *testServer) CallPcreInvalid(ctx context.Context, req *pb.Request, rsp *pb.Response) error {
	rsp.Msg = "Hello " + req.Uuid
	return nil
}

func initial(t *testing.T) (server.Server, client.Client) {
	r := rmemory.NewRegistry()

	// create a new client
	s := gsrv.NewServer(
		server.Name("foo"),
		server.Registry(r),
	)

	rtr := regRouter.NewRouter(
		rt.Registry(r),
	)

	// create a new server
	c := gcli.NewClient(
		client.Router(rtr),
	)

	h := &testServer{}
	pb.RegisterTestHandler(s, h)

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	return s, c
}

func check(t *testing.T, addr string, path string, expected string) {
	req, err := http.NewRequest("POST", fmt.Sprintf(path, addr), nil)
	if err != nil {
		t.Fatalf("Failed to created http.Request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	rsp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("Failed to created http.Request: %v", err)
	}
	defer rsp.Body.Close()

	buf, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		t.Fatal(err)
	}

	jsonMsg := expected
	if string(buf) != jsonMsg {
		t.Fatalf("invalid message received, parsing error %s != %s", buf, jsonMsg)
	}
}

func TestRouterRegistryPcre(t *testing.T) {
	s, c := initial(t)
	defer s.Stop()

	router := rregistry.NewRouter(
		router.WithHandler(rpc.Handler),
		router.WithRegistry(s.Options().Registry),
	)
	hrpc := rpc.NewHandler(
		handler.WithClient(c),
		handler.WithRouter(router),
	)
	hsrv := &http.Server{
		Handler:        hrpc,
		Addr:           "127.0.0.1:6543",
		WriteTimeout:   15 * time.Second,
		ReadTimeout:    15 * time.Second,
		IdleTimeout:    20 * time.Second,
		MaxHeaderBytes: 1024 * 1024 * 1, // 1Mb
	}

	go func() {
		log.Println(hsrv.ListenAndServe())
	}()

	defer hsrv.Close()
	time.Sleep(1 * time.Second)
	check(t, hsrv.Addr, "http://%s/api/v0/test/call/TEST", `{"msg":"Hello TEST"}`)
}

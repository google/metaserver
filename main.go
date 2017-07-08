/*

Copyright 2017 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

*/

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/golang/glog"
	"github.com/google/credstore/client"
	"github.com/google/go-microservice-helpers/client"
	"github.com/google/go-microservice-helpers/tracing"
	pubkeystorepb "github.com/google/pubkeystore/api"
	vmregistrypb "github.com/google/vmregistry/api"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	opentracing "github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/google/metaserver/ec2server"
	"github.com/google/metaserver/noclserver"
)

var (
	listenAddress = flag.String("listen", "127.0.0.1:8080", "listen address")

	credStoreAddress = flag.String("credstore-address", "", "credstore grpc address")
	credStoreCA      = flag.String("credstore-ca", "", "credstore server ca")

	vmregistryAddress = flag.String("vmregistry-address", "127.0.0.1:9000", "vm registry grpc address")
	vmregistryCA      = flag.String("vmregistry-ca", "", "vm registry server ca")

	pubkeystoreAddress = flag.String("pubkeystore-address", "127.0.0.1:9001", "pubkeystore grpc address")
	pubkeystoreCA      = flag.String("pubkeystore-ca", "", "pubkeystore server ca")

	logHTTP              = flag.Bool("log-http", true, "log http requests to stdout")
	apiMode              = flag.String("api-mode", "ec2", "serving mode, either 'ec2' or 'nocloud'")
	userDataTemplateFile = flag.String("userdata-template-file", "", "template file for user-data")
	dnsName              = flag.String("dns-name", "", "dns name for nodes")
)

type vmregistryBearerClient struct {
	cli vmregistrypb.VMRegistryClient
	tok string
}

func (c vmregistryBearerClient) List(ctx context.Context, in *vmregistrypb.ListVMRequest, opts ...grpc.CallOption) (*vmregistrypb.ListVMReply, error) {
	return c.cli.List(client.WithBearerToken(ctx, c.tok), in, opts...)
}
func (c vmregistryBearerClient) Find(ctx context.Context, in *vmregistrypb.FindRequest, opts ...grpc.CallOption) (*vmregistrypb.VM, error) {
	return c.cli.Find(client.WithBearerToken(ctx, c.tok), in, opts...)
}
func (c vmregistryBearerClient) Create(ctx context.Context, in *vmregistrypb.CreateRequest, opts ...grpc.CallOption) (*vmregistrypb.VM, error) {
	return c.cli.Create(client.WithBearerToken(ctx, c.tok), in, opts...)
}
func (c vmregistryBearerClient) Destroy(ctx context.Context, in *vmregistrypb.DestroyRequest, opts ...grpc.CallOption) (*vmregistrypb.DestroyReply, error) {
	return c.cli.Destroy(client.WithBearerToken(ctx, c.tok), in, opts...)
}

func newVMRegistryClient(credstoreClient *client.CredstoreClient) (vmregistrypb.VMRegistryClient, error) {
	tok, err := credstoreClient.GetTokenForRemote(context.Background(), *vmregistryAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get vmregistry token: %v", err)
	}

	conn, err := clienthelpers.NewGRPCConn(*vmregistryAddress, *vmregistryCA, "", "")
	if err != nil {
		return nil, err
	}
	cli := &vmregistryBearerClient{
		cli: vmregistrypb.NewVMRegistryClient(conn),
		tok: tok,
	}
	return cli, nil
}

type pubkeystoreBearerClient struct {
	cli pubkeystorepb.PubkeyStoreClient
	tok string
}

func (c pubkeystoreBearerClient) GetKeys(ctx context.Context, in *pubkeystorepb.GetKeysRequest, opts ...grpc.CallOption) (*pubkeystorepb.GetKeysReply, error) {
	return c.cli.GetKeys(client.WithBearerToken(ctx, c.tok), in, opts...)
}

func newPubkeyStoreClient(credstoreClient *client.CredstoreClient) (pubkeystorepb.PubkeyStoreClient, error) {
	tok, err := credstoreClient.GetTokenForRemote(context.Background(), *pubkeystoreAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get pubkeystore token: %v", err)
	}

	conn, err := clienthelpers.NewGRPCConn(*pubkeystoreAddress, *pubkeystoreCA, "", "")
	if err != nil {
		return nil, err
	}
	cli := &pubkeystoreBearerClient{
		cli: pubkeystorepb.NewPubkeyStoreClient(conn),
		tok: tok,
	}
	return cli, nil
}

func main() {
	flag.Parse()
	defer glog.Flush()

	err := tracing.InitTracer(*listenAddress, "metaserver")
	if err != nil {
		glog.Fatalf("failed to init tracing interface: %v", err)
	}

	credstoreClient, err := client.NewCredstoreClient(context.Background(), *credStoreAddress, *credStoreCA)
	if err != nil {
		glog.Fatalf("failed to init credstore: %v", err)
	}

	vmregClient, err := newVMRegistryClient(credstoreClient)
	if err != nil {
		glog.Fatalf("failed to connect to vmregistry: %v", err)
	}

	pubkeystoreClient, err := newPubkeyStoreClient(credstoreClient)
	if err != nil {
		glog.Fatalf("failed to connect to pubkeystore: %v", err)
	}

	userDataBytes, err := ioutil.ReadFile(*userDataTemplateFile)
	if err != nil {
		glog.Fatalf("failed to read userdata template file: %v", err)
	}

	var r *mux.Router
	if *apiMode == "ec2" {
		svr := ec2server.NewServer(vmregClient, pubkeystoreClient, string(userDataBytes))
		r = ec2server.NewRouter(&svr)
	} else if *apiMode == "nocloud" {
		svr := noclserver.NewServer(vmregClient, pubkeystoreClient, *dnsName, string(userDataBytes))
		r = noclserver.NewRouter(&svr)
	} else {
		glog.Fatalf("unsupported api mode: %s", *apiMode)
	}

	if *logHTTP {
		loggedRouter := handlers.LoggingHandler(os.Stdout, r)
		http.Handle("/", loggedRouter)
	} else {
		http.Handle("/", r)
	}

	glog.Infof("serving on %v", *listenAddress)
	err = http.ListenAndServe(*listenAddress,
		nethttp.Middleware(
			opentracing.GlobalTracer(),
			http.DefaultServeMux,
			nethttp.OperationNameFunc(func(rq *http.Request) string {
				return "HTTP " + rq.Method + " " + rq.URL.Path
			})))
	glog.Fatalf("failed to listen and serve: %v", err)
}

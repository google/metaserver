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

package ec2server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
)

// NewRouter returns a router bound to Server.
func NewRouter(s *Server) *mux.Router {
	rootRouter := mux.NewRouter()
	r := rootRouter.PathPrefix("/2009-04-04").Subrouter()

	r.HandleFunc("/meta-data/", func(w http.ResponseWriter, rq *http.Request) {
		ctx := rq.Context()
		sp, ctx := opentracing.StartSpanFromContext(ctx, "HandleMetadata")
		defer sp.Finish()

		s.HandleMetadata(w)
	})

	r.HandleFunc("/meta-data/hostname", func(w http.ResponseWriter, rq *http.Request) {
		ctx := rq.Context()
		sp, ctx := opentracing.StartSpanFromContext(ctx, "HandleHostname")
		defer sp.Finish()

		remoteIP := strings.SplitN(rq.RemoteAddr, ":", 2)[0]
		sp.LogFields(log.String("client", rq.RemoteAddr))
		s.HandleHostname(ctx, w, remoteIP)
	})

	r.HandleFunc("/meta-data/instance-id", func(w http.ResponseWriter, rq *http.Request) {
		ctx := rq.Context()
		sp, ctx := opentracing.StartSpanFromContext(ctx, "HandleInstanceID")
		defer sp.Finish()

		remoteIP := strings.SplitN(rq.RemoteAddr, ":", 2)[0]
		sp.LogFields(log.String("client", rq.RemoteAddr))
		s.HandleInstanceID(ctx, w, remoteIP)
	})

	r.HandleFunc("/meta-data/public-keys", func(w http.ResponseWriter, rq *http.Request) {
		http.Redirect(w, rq, "http://169.254.169.254/2009-04-04/meta-data/public-keys/", 301)
	})

	r.HandleFunc("/meta-data/public-keys/", func(w http.ResponseWriter, rq *http.Request) {
		ctx := rq.Context()
		sp, ctx := opentracing.StartSpanFromContext(ctx, "HandlePublicKeys")
		defer sp.Finish()

		remoteIP := strings.SplitN(rq.RemoteAddr, ":", 2)[0]
		sp.LogFields(log.String("client", rq.RemoteAddr))
		s.HandlePublicKeys(ctx, w, remoteIP)
	})

	r.HandleFunc("/meta-data/public-keys/{key}/", func(w http.ResponseWriter, rq *http.Request) {
		ctx := rq.Context()
		sp, ctx := opentracing.StartSpanFromContext(ctx, "HandlePublicKey")
		defer sp.Finish()

		remoteIP := strings.SplitN(rq.RemoteAddr, ":", 2)[0]
		keyIdx, err := strconv.Atoi(mux.Vars(rq)["key"])
		if err != nil {
			sp.SetTag("error", true)
			glog.Errorf("cannot parse key index: %v", err)
			w.WriteHeader(500)
			return
		}

		sp.LogFields(log.String("client", rq.RemoteAddr))
		s.HandlePublicKey(ctx, w, remoteIP, keyIdx)
	})
	r.HandleFunc("/meta-data/public-keys/{key}/openssh-key", func(w http.ResponseWriter, rq *http.Request) {
		ctx := rq.Context()
		sp, ctx := opentracing.StartSpanFromContext(ctx, "HandlePublicKeyData")
		defer sp.Finish()

		remoteIP := strings.SplitN(rq.RemoteAddr, ":", 2)[0]
		keyIdx, err := strconv.Atoi(mux.Vars(rq)["key"])
		if err != nil {
			sp.SetTag("error", true)
			glog.Errorf("cannot parse key index: %v", err)
			w.WriteHeader(500)
			return
		}

		sp.LogFields(log.String("client", rq.RemoteAddr), log.Int("key_idx", keyIdx))
		s.HandlePublicKeyData(ctx, w, remoteIP, keyIdx)
	})

	return rootRouter
}

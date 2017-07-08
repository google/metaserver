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

package noclserver

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
)

// NewRouter returns a router bound to Server.
func NewRouter(s *Server) *mux.Router {
	r := mux.NewRouter()

	r.HandleFunc("/meta-data", func(w http.ResponseWriter, rq *http.Request) {
		ctx := rq.Context()
		sp, ctx := opentracing.StartSpanFromContext(ctx, "HandleMetadata")
		defer sp.Finish()

		remoteIP := strings.SplitN(rq.RemoteAddr, ":", 2)[0]
		sp.LogFields(log.String("client", rq.RemoteAddr))
		s.HandleMetadata(ctx, w, remoteIP)
	})

	r.HandleFunc("/user-data", func(w http.ResponseWriter, rq *http.Request) {
		ctx := rq.Context()
		sp, ctx := opentracing.StartSpanFromContext(ctx, "HandleUserdata")
		defer sp.Finish()

		remoteIP := strings.SplitN(rq.RemoteAddr, ":", 2)[0]
		sp.LogFields(log.String("client", rq.RemoteAddr))
		s.HandleUserdata(ctx, w, remoteIP)
	})

	return r
}

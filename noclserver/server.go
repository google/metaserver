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
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/golang/glog"
	"gopkg.in/yaml.v2"

	pubkeystore "github.com/google/pubkeystore/api"
	vmregistrypb "github.com/google/vmregistry/api"
)

// Server implements http serving handlers.
type Server struct {
	vmregClient       vmregistrypb.VMRegistryClient
	pubkeystoreClient pubkeystore.PubkeyStoreClient

	dnsName          string
	userDataTemplate *template.Template
}

// NewServer creates a new Server.
func NewServer(
	vmregClient vmregistrypb.VMRegistryClient,
	pubkeystoreClient pubkeystore.PubkeyStoreClient,
	dnsName string,
	userDataTemplate string) Server {
	return Server{
		vmregClient:       vmregClient,
		pubkeystoreClient: pubkeystoreClient,
		dnsName:           dnsName,
		userDataTemplate:  template.Must(template.New("userdata").Parse(strings.TrimSpace(userDataTemplate))),
	}
}

// HandleMetadata handles /meta-data request.
func (s Server) HandleMetadata(ctx context.Context, w http.ResponseWriter, ip string) {
	vm, err := s.vmregClient.Find(ctx, &vmregistrypb.FindRequest{FindBy: vmregistrypb.FindRequest_IP, Value: ip})
	if err != nil {
		glog.Errorf("failed to get vm by ip %s: %v", ip, err)
		w.WriteHeader(500)
		return
	}

	pkeys, err := s.pubkeystoreClient.GetKeys(ctx, &pubkeystore.GetKeysRequest{VmName: vm.Name})
	if err != nil {
		glog.Errorf("failed to get public keys for vm %s: %v", vm.Name, err)
		w.WriteHeader(500)
		return
	}

	keys := make([]string, len(pkeys.Keys))
	for i, key := range pkeys.Keys {
		keys[i] = fmt.Sprintf("%s %s %s", key.Algo, key.Pubkey, key.Comment)
	}

	md := Metadata{
		LocalHostname: vm.Name,
		InstanceID:    "i-" + vm.Name,
		PublicKeys:    keys,
	}

	jsonData, err := yaml.Marshal(md)
	if err != nil {
		glog.Errorf("failed to serialize json for vm %s md %v: %v", vm.Name, md, err)
		w.WriteHeader(500)
		return
	}

	w.Write(jsonData)
}

// HandleUserdata handles /user-data request.
func (s Server) HandleUserdata(ctx context.Context, w http.ResponseWriter, ip string) {
	vm, err := s.vmregClient.Find(ctx, &vmregistrypb.FindRequest{FindBy: vmregistrypb.FindRequest_IP, Value: ip})
	if err != nil {
		glog.Errorf("failed to get vm by ip %s: %v", ip, err)
		w.WriteHeader(500)
		return
	}

	pkeys, err := s.pubkeystoreClient.GetKeys(ctx, &pubkeystore.GetKeysRequest{VmName: vm.Name})
	if err != nil {
		glog.Errorf("failed to get public keys for vm %s: %v", vm.Name, err)
		w.WriteHeader(500)
		return
	}

	keys := make([]string, len(pkeys.Keys))
	for i, key := range pkeys.Keys {
		keys[i] = fmt.Sprintf("%s %s %s", key.Algo, key.Pubkey, key.Comment)
	}

	var userDataBuffer bytes.Buffer
	s.userDataTemplate.Execute(&userDataBuffer, struct {
		Hostname   string
		PublicKeys []string
	}{
		Hostname:   vm.Name + "." + s.dnsName,
		PublicKeys: keys,
	})
	userData := userDataBuffer.String()

	w.Write([]byte(userData))
}

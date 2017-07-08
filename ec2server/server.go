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
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/golang/glog"

	pubkeystorepb "github.com/google/pubkeystore/api"
	vmregistrypb "github.com/google/vmregistry/api"
)

// Server implements http serving handlers.
type Server struct {
	vmregClient       vmregistrypb.VMRegistryClient
	pubkeystoreClient pubkeystorepb.PubkeyStoreClient

	userDataTemplate *template.Template
}

// NewServer creates a new Server.
func NewServer(vmregClient vmregistrypb.VMRegistryClient, pubkeystoreClient pubkeystorepb.PubkeyStoreClient, userDataTemplate string) Server {
	return Server{
		vmregClient:       vmregClient,
		pubkeystoreClient: pubkeystoreClient,
		userDataTemplate:  template.Must(template.New("userdata").Parse(strings.TrimSpace(userDataTemplate))),
	}
}

// HandleMetadata handles /meta-data/ request.
func (s Server) HandleMetadata(w http.ResponseWriter) {
	renderList(metadataKeys, w)
}

// HandleHostname handles /meta-data/hostname request.
func (s Server) HandleHostname(ctx context.Context, w http.ResponseWriter, ip string) {
	vm, err := s.vmregClient.Find(ctx, &vmregistrypb.FindRequest{FindBy: vmregistrypb.FindRequest_IP, Value: ip})
	if err != nil {
		glog.Errorf("failed to get vm by ip %s: %v", ip, err)
		w.WriteHeader(500)
		return
	}
	renderString(vm.Name, w)
}

// HandleInstanceID handles /meta-data/instance-id request.
func (s Server) HandleInstanceID(ctx context.Context, w http.ResponseWriter, ip string) {
	vm, err := s.vmregClient.Find(ctx, &vmregistrypb.FindRequest{FindBy: vmregistrypb.FindRequest_IP, Value: ip})
	if err != nil {
		glog.Errorf("failed to get vm by ip %s: %v", ip, err)
		w.WriteHeader(500)
		return
	}

	instanceID := "i-" + vm.Name

	renderString(instanceID, w)
}

// HandlePublicKeys handles /meta-data/public-keys/ request.
func (s Server) HandlePublicKeys(ctx context.Context, w http.ResponseWriter, ip string) {
	vm, err := s.vmregClient.Find(ctx, &vmregistrypb.FindRequest{FindBy: vmregistrypb.FindRequest_IP, Value: ip})
	if err != nil {
		glog.Errorf("failed to get vm by ip %s: %v", ip, err)
		w.WriteHeader(500)
		return
	}

	pkeys, err := s.pubkeystoreClient.GetKeys(ctx, &pubkeystorepb.GetKeysRequest{VmName: vm.Name})
	if err != nil {
		glog.Errorf("failed to get public keys for vm %s: %v", vm.Name, err)
		w.WriteHeader(500)
		return
	}

	repl := make([]string, len(pkeys.Keys))

	for i, k := range pkeys.Keys {
		repl[i] = fmt.Sprintf("%d=%s", i, k.Name)
	}

	renderList(repl, w)
}

// HandlePublicKey handles /meta-data/public-keys/{key} request.
func (s Server) HandlePublicKey(ctx context.Context, w http.ResponseWriter, ip string, keyIdx int) {
	vm, err := s.vmregClient.Find(ctx, &vmregistrypb.FindRequest{FindBy: vmregistrypb.FindRequest_IP, Value: ip})
	if err != nil {
		glog.Errorf("failed to get vm by ip %s: %v", ip, err)
		w.WriteHeader(500)
		return
	}

	pkeys, err := s.pubkeystoreClient.GetKeys(ctx, &pubkeystorepb.GetKeysRequest{VmName: vm.Name})
	if err != nil {
		glog.Errorf("failed to get public keys for vm %s: %v", vm.Name, err)
		w.WriteHeader(500)
		return
	}

	if keyIdx >= len(pkeys.Keys) {
		glog.Errorf("cannot find key with index %d for %s (got %d keys)", keyIdx, vm.Name, len(pkeys.Keys))
		w.WriteHeader(500)
		return
	}

	renderString("openssh-key", w)
}

// HandlePublicKeyData handles /meta-data/public-keys/{key}/openssh-key request.
func (s Server) HandlePublicKeyData(ctx context.Context, w http.ResponseWriter, ip string, keyIdx int) {
	vm, err := s.vmregClient.Find(ctx, &vmregistrypb.FindRequest{FindBy: vmregistrypb.FindRequest_IP, Value: ip})
	if err != nil {
		glog.Errorf("failed to get vm by ip %s: %v", ip, err)
		w.WriteHeader(500)
		return
	}

	pkeys, err := s.pubkeystoreClient.GetKeys(ctx, &pubkeystorepb.GetKeysRequest{VmName: vm.Name})
	if err != nil {
		glog.Errorf("failed to get public keys for vm %s: %v", vm.Name, err)
		w.WriteHeader(500)
		return
	}

	if keyIdx >= len(pkeys.Keys) {
		glog.Errorf("cannot find key with index %d for %s (got %d keys)", keyIdx, vm.Name, len(pkeys.Keys))
		w.WriteHeader(500)
		return
	}

	key := pkeys.Keys[keyIdx]
	keyString := fmt.Sprintf("%s %s %s", key.Algo, key.Pubkey, key.Comment)

	renderString(keyString, w)
}

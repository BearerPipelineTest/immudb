/*
Copyright 2019 vChain, Inc.

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

package server

import (
	"context"
	"net"

	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"github.com/codenotary/immudb/pkg/db"
	"github.com/codenotary/immudb/pkg/logger"
	"github.com/codenotary/immudb/pkg/schema"
)

func Run(options *Options) error {
	listener, err := net.Listen(options.Network, options.Bind())
	if err != nil {
		return err
	}
	b, err := makeBadger(options.Dir, options.DbName)
	if err != nil {
		return err
	}
	server := &ImmuServer{
		Topic:  db.NewTopic(b),
		Logger: logger.DefaultLogger}
	server.Logger.Infof("starting immudb %v", options)
	gRpcServer := grpc.NewServer()
	schema.RegisterImmuServiceServer(gRpcServer, server)
	return gRpcServer.Serve(listener)
}

func (s *ImmuServer) Set(ctx context.Context, sr *schema.SetRequest) (*empty.Empty, error) {
	s.Logger.Debugf("set %s %d bytes", sr.Key, len(sr.Value))
	if err := s.Topic.Set(sr.Key, sr.Value); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

func (s *ImmuServer) Get(ctx context.Context, gr *schema.GetRequest) (*schema.GetResponse, error) {
	value, err := s.Topic.Get(gr.Key)
	s.Logger.Debugf("get %s %d bytes", gr.Key, len(value))
	if err != nil {
		return nil, err
	}
	return &schema.GetResponse{Value: value}, nil
}

func makeBadger(dir string, name string) (*badger.DB, error) {
	opts := badger.
		DefaultOptions(dir + "/" + name).
		WithTableLoadingMode(options.LoadToRAM).
		WithCompressionType(options.None).
		WithSyncWrites(false).
		WithEventLogging(false)
	return badger.Open(opts)
}

package nri

import (
	"context"
	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	"github.com/koordinator-sh/koordinator/pkg/runtimeproxy-nri/dispatcher"
	"os"
	"strings"
	"time"
)

const (
	defaultTimeout = 5 * time.Second
)

type NRIServer struct {
	hookDispatcher *dispatcher.RuntimeHookDispatcher
}

func NewNRIServer() *NRIServer {
	s := &NRIServer{
		hookDispatcher: dispatcher.NewRuntimeDispatcher(),
	}
	return s
}

func (s NRIServer) Run() error {
	go s.initNRI()
	return nil
}

func (s *NRIServer) Name() string {
	return "RuntimeManagerNRIServer"
}

// start NRI stub
func (s *NRIServer) initNRI() error {
	var (
		pluginName = "NRI-RuntimeProxy"
		pluginIdx  = "00"
		events     = "all"
		opts       []stub.Option
		err        error
	)

	if pluginName != "" {
		opts = append(opts, stub.WithPluginName(pluginName))
	}
	if pluginIdx != "" {
		opts = append(opts, stub.WithPluginIdx(pluginIdx))
	}

	p := &plugin{}
	if p.mask, err = api.ParseEventMask(events); err != nil {
		log.Fatalf("failed to parse events: %v", err)
	}
	cfg.Events = strings.Split(events, ",")

	if p.stub, err = stub.New(p, append(opts, stub.WithOnClose(p.onClose))...); err != nil {
		log.Fatalf("failed to create plugin stub: %v", err)
	}

	err = p.stub.Run(context.Background())
	if err != nil {
		log.Errorf("plugin exited with error %v", err)
		os.Exit(1)
	}
	return nil
}

// failOver will sync all pods and containers and store
func (s *NRIServer) failOver() error {
	return nil
}

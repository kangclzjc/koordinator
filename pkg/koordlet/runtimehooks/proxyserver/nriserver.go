package proxyserver

import (
	"context"
	"fmt"
	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/runtimehooks/hooks"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/runtimehooks/protocol"
	rmconfig "github.com/koordinator-sh/koordinator/pkg/runtimeproxy/config"
	"k8s.io/klog/v2"
	"log"
	"os"
	"sigs.k8s.io/yaml"
	"strings"
)

var (
	_          = stub.ConfigureInterface(&nriServer{})
	pluginName = "koordlet_nri"
	pluginIdx  = "00"
	events     = "RunPodSandbox,UpdateContainer,StartContainer"
	cfg        nriconfig
	opts       []stub.Option
	err        error
)

type nriconfig struct {
	LogFile       string   `json:"logFile"`
	Events        []string `json:"events"`
	AddAnnotation string   `json:"addAnnotation"`
	SetAnnotation string   `json:"setAnnotation"`
	AddEnv        string   `json:"addEnv"`
	SetEnv        string   `json:"setEnv"`
}

type nriServer struct {
	stub    stub.Stub
	mask    stub.EventMask
	options Options // server options
}

func NewNriServer() (*nriServer, error) {
	opts = append(opts, stub.WithPluginName(pluginName))
	if pluginIdx != "" {
		opts = append(opts, stub.WithPluginIdx(pluginIdx))
	}
	p := &nriServer{}
	if p.mask, err = api.ParseEventMask(events); err != nil {
		log.Fatalf("failed to parse events: %v", err)
	}
	cfg.Events = strings.Split(events, ",")

	if p.stub, err = stub.New(p, append(opts, stub.WithOnClose(p.onClose))...); err != nil {
		log.Fatalf("failed to create plugin stub: %v", err)
	}

	return p, err
}

func (s *nriServer) Setup() error {
	err = s.stub.Run(context.Background())
	if err != nil {
		klog.Errorf("plugin exited with error %v", err)
		os.Exit(1)
	}
	return nil
}

func (s *nriServer) Start() error {
	return nil
}

func (s *nriServer) Stop() {

}

func (s *nriServer) Register() error {
	return nil
}

func (p *nriServer) Configure(config, runtime, version string) (stub.EventMask, error) {
	klog.Infof("got configuration data: %q from runtime %s %s", config, runtime, version)
	if config == "" {
		return p.mask, nil
	}

	err := yaml.Unmarshal([]byte(config), &cfg)
	if err != nil {
		return 0, fmt.Errorf("failed to parse provided configuration: %w", err)
	}

	p.mask, err = api.ParseEventMask(cfg.Events...)
	if err != nil {
		return 0, fmt.Errorf("failed to parse events in configuration: %w", err)
	}

	return p.mask, nil
}

func (p *nriServer) Synchronize(pods []*api.PodSandbox, containers []*api.Container) ([]*api.ContainerUpdate, error) {
	return nil, nil
}

func (p *nriServer) Shutdown() {
}

func (p *nriServer) RunPodSandbox(pod *api.PodSandbox) error {
	podCtx := &protocol.PodContext{}
	podCtx.FromNri(pod)
	err := hooks.RunHooks(p.options.PluginFailurePolicy, rmconfig.PreRunPodSandbox, podCtx)
	if err != nil {
		klog.Errorf("hooks run error: %v", err)
	}
	podCtx.NriDone()
	return nil
}

func (p *nriServer) CreateContainer(pod *api.PodSandbox, container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	containerCtx := &protocol.ContainerContext{}
	containerCtx.FromNri(pod, container)
	err := hooks.RunHooks(p.options.PluginFailurePolicy, rmconfig.PreCreateContainer, containerCtx)
	if err != nil {
		klog.Errorf("run hooks error: %v", err)
	}

	adjust := &api.ContainerAdjustment{}
	if containerCtx.Response.Resources.CPUSet != nil {
		adjust.SetLinuxCPUSetCPUs(*containerCtx.Response.Resources.CPUSet)
	}

	if containerCtx.Response.Resources.CFSQuota != nil {
		adjust.SetLinuxCPUQuota(*containerCtx.Response.Resources.CFSQuota)
	}

	if containerCtx.Response.Resources.CPUShares != nil {
		adjust.SetLinuxCPUShares(uint64(*containerCtx.Response.Resources.CPUShares))
	}

	if containerCtx.Response.Resources.MemoryLimit != nil {
		adjust.SetLinuxMemoryLimit(*containerCtx.Response.Resources.MemoryLimit)
	}

	if containerCtx.Response.AddContainerEnvs != nil {
		for k, v := range containerCtx.Response.AddContainerEnvs {
			adjust.AddEnv(k, v)
		}
	}
	return adjust, nil, nil
}

func (p *nriServer) UpdateContainer(pod *api.PodSandbox, container *api.Container) ([]*api.ContainerUpdate, error) {
	containerCtx := &protocol.ContainerContext{}
	containerCtx.FromNri(pod, container)
	err := hooks.RunHooks(p.options.PluginFailurePolicy, rmconfig.PreCreateContainer, containerCtx)
	if err != nil {
		klog.Errorf("run hooks error: %v", err)
	}

	update := &api.ContainerUpdate{}
	if containerCtx.Response.Resources.CPUSet != nil {
		update.SetLinuxCPUSetCPUs(*containerCtx.Response.Resources.CPUSet)
	}

	if containerCtx.Response.Resources.CFSQuota != nil {
		update.SetLinuxCPUQuota(*containerCtx.Response.Resources.CFSQuota)
	}

	if containerCtx.Response.Resources.CPUShares != nil {
		update.SetLinuxCPUShares(uint64(*containerCtx.Response.Resources.CPUShares))
	}

	if containerCtx.Response.Resources.MemoryLimit != nil {
		update.SetLinuxMemoryLimit(*containerCtx.Response.Resources.MemoryLimit)
	}

	return []*api.ContainerUpdate{update}, nil
}

func (p *nriServer) onClose() {
	os.Exit(0)
}

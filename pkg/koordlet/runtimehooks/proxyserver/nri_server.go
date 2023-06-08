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
	events     string
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

func NewNriServer() *nriServer {
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

	return p
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

	oldCfg := cfg
	err := yaml.Unmarshal([]byte(config), &cfg)
	if err != nil {
		return 0, fmt.Errorf("failed to parse provided configuration: %w", err)
	}

	p.mask, err = api.ParseEventMask(cfg.Events...)
	if err != nil {
		return 0, fmt.Errorf("failed to parse events in configuration: %w", err)
	}

	if cfg.LogFile != oldCfg.LogFile {
		f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			klog.Errorf("failed to open log file %q: %v", cfg.LogFile, err)
			return 0, fmt.Errorf("failed to open log file %q: %w", cfg.LogFile, err)
		}
		log.SetOutput(f)
	}

	return p.mask, nil
}

func (p *nriServer) Synchronize(pods []*api.PodSandbox, containers []*api.Container) ([]*api.ContainerUpdate, error) {
	//configEgressGroup
	//configingressGroup
	return nil, nil
}

func (p *nriServer) Shutdown() {
	dump("Shutdown")
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

func (p *nriServer) StopPodSandbox(pod *api.PodSandbox) error {
	dump("StopPodSandbox", "pod", pod)
	return nil
}

func (p *nriServer) RemovePodSandbox(pod *api.PodSandbox) error {
	dump("RemovePodSandbox", "pod", pod)
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

func (p *nriServer) PostCreateContainer(pod *api.PodSandbox, container *api.Container) error {
	dump("PostCreateContainer", "pod", pod, "container", container)
	return nil
}

func (p *nriServer) StartContainer(pod *api.PodSandbox, container *api.Container) error {
	dump("StartContainer", "pod", pod, "container", container)
	return nil
}

func (p *nriServer) PostStartContainer(pod *api.PodSandbox, container *api.Container) error {
	dump("PostStartContainer", "pod", pod, "container", container)
	return nil
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

func (p *nriServer) PostUpdateContainer(pod *api.PodSandbox, container *api.Container) error {
	dump("PostUpdateContainer", "pod", pod, "container", container)
	return nil
}

func (p *nriServer) StopContainer(pod *api.PodSandbox, container *api.Container) ([]*api.ContainerUpdate, error) {
	dump("StopContainer", "pod", pod, "container", container)
	return nil, nil
}

func (p *nriServer) RemoveContainer(pod *api.PodSandbox, container *api.Container) error {
	dump("RemoveContainer", "pod", pod, "container", container)
	return nil
}

func (p *nriServer) onClose() {
	os.Exit(0)
}

// Dump one or more objects, with an optional global prefix and per-object tags.
func dump(args ...interface{}) {
	var (
		prefix string
		idx    int
	)

	if len(args)&0x1 == 1 {
		prefix = args[0].(string)
		idx++
	}

	for ; idx < len(args)-1; idx += 2 {
		tag, obj := args[idx], args[idx+1]
		msg, err := yaml.Marshal(obj)
		if err != nil {
			klog.Infof("%s: %s: failed to dump object: %v", prefix, tag, err)
			continue
		}

		if prefix != "" {
			klog.Infof("%s: %s:", prefix, tag)
			for _, line := range strings.Split(strings.TrimSpace(string(msg)), "\n") {
				klog.Infof("%s:    %s", prefix, line)
			}
		} else {
			klog.Infof("%s:", tag)
			for _, line := range strings.Split(strings.TrimSpace(string(msg)), "\n") {
				klog.Infof("  %s", line)
			}
		}
	}
}

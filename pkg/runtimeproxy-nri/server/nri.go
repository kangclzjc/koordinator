package nri

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"k8s.io/klog/v2"
	"os"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
)

type config struct {
	LogFile       string   `json:"logFile"`
	Events        []string `json:"events"`
	AddAnnotation string   `json:"addAnnotation"`
	SetAnnotation string   `json:"setAnnotation"`
	AddEnv        string   `json:"addEnv"`
	SetEnv        string   `json:"setEnv"`
}

type plugin struct {
	stub stub.Stub
	mask stub.EventMask
}

var (
	cfg config
	log *logrus.Logger
	_   = stub.ConfigureInterface(&plugin{})
)

func (p *plugin) failOver(pods []*api.PodSandbox, containers []*api.Container) error {
	//
	//for _, pod := range pods {
	//	podResourceExecutor := cri_resource_executor.NewPodResourceExecutor()
	//	podResourceExecutor.ParsePod(pod)
	//	podResourceExecutor.ResourceCheckPoint(&runtimeapi.RunPodSandboxResponse{
	//		PodSandboxId: pod.GetId(),
	//	})
	//}
	return nil
}

func (p *plugin) Configure(config, runtime, version string) (stub.EventMask, error) {
	log.Infof("got configuration data: %q from runtime %s %s", config, runtime, version)
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
			log.Errorf("failed to open log file %q: %v", cfg.LogFile, err)
			return 0, fmt.Errorf("failed to open log file %q: %w", cfg.LogFile, err)
		}
		log.SetOutput(f)
	}

	return p.mask, nil
}

func (p *plugin) Synchronize(pods []*api.PodSandbox, containers []*api.Container) ([]*api.ContainerUpdate, error) {
	p.failOver(pods, containers)
	klog.Infof("do failOver done")
	return nil, nil
}

func (p *plugin) Shutdown() {
	dump("Shutdown")
}

func (p *plugin) RunPodSandbox(pod *api.PodSandbox) error {
	dump("RunPodSandbox", "pod", pod)
	return nil
}

func (p *plugin) StopPodSandbox(pod *api.PodSandbox) error {
	dump("StopPodSandbox", "pod", pod)
	return nil
}

func (p *plugin) RemovePodSandbox(pod *api.PodSandbox) error {
	dump("RemovePodSandbox", "pod", pod)
	return nil
}

func (p *plugin) CreateContainer(pod *api.PodSandbox, container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	dump("CreateContainer", "pod", pod, "container", container)

	adjust := &api.ContainerAdjustment{}

	if cfg.AddAnnotation != "" {
		adjust.AddAnnotation(cfg.AddAnnotation, fmt.Sprintf("logger-pid-%d", os.Getpid()))
	}
	if cfg.SetAnnotation != "" {
		adjust.RemoveAnnotation(cfg.SetAnnotation)
		adjust.AddAnnotation(cfg.SetAnnotation, fmt.Sprintf("logger-pid-%d", os.Getpid()))
	}
	if cfg.AddEnv != "" {
		adjust.AddEnv(cfg.AddEnv, fmt.Sprintf("logger-pid-%d", os.Getpid()))
	}
	if cfg.SetEnv != "" {
		adjust.RemoveEnv(cfg.SetEnv)
		adjust.AddEnv(cfg.SetEnv, fmt.Sprintf("logger-pid-%d", os.Getpid()))
	}

	return adjust, nil, nil
}

func (p *plugin) PostCreateContainer(pod *api.PodSandbox, container *api.Container) error {
	dump("PostCreateContainer", "pod", pod, "container", container)
	return nil
}

func (p *plugin) StartContainer(pod *api.PodSandbox, container *api.Container) error {
	dump("StartContainer", "pod", pod, "container", container)
	return nil
}

func (p *plugin) PostStartContainer(pod *api.PodSandbox, container *api.Container) error {
	dump("PostStartContainer", "pod", pod, "container", container)
	return nil
}

func (p *plugin) UpdateContainer(pod *api.PodSandbox, container *api.Container) ([]*api.ContainerUpdate, error) {
	dump("UpdateContainer", "pod", pod, "container", container)
	return nil, nil
}

func (p *plugin) PostUpdateContainer(pod *api.PodSandbox, container *api.Container) error {
	dump("PostUpdateContainer", "pod", pod, "container", container)
	return nil
}

func (p *plugin) StopContainer(pod *api.PodSandbox, container *api.Container) ([]*api.ContainerUpdate, error) {
	dump("StopContainer", "pod", pod, "container", container)
	return nil, nil
}

func (p *plugin) RemoveContainer(pod *api.PodSandbox, container *api.Container) error {
	dump("RemoveContainer", "pod", pod, "container", container)
	return nil
}

func (p *plugin) onClose() {
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
			log.Infof("%s: %s: failed to dump object: %v", prefix, tag, err)
			continue
		}

		if prefix != "" {
			log.Infof("%s: %s:", prefix, tag)
			for _, line := range strings.Split(strings.TrimSpace(string(msg)), "\n") {
				log.Infof("%s:    %s", prefix, line)
			}
		} else {
			log.Infof("%s:", tag)
			for _, line := range strings.Split(strings.TrimSpace(string(msg)), "\n") {
				log.Infof("  %s", line)
			}
		}
	}
}

package resexecutor

import (
	"github.com/koordinator-sh/koordinator/apis/runtime/v1alpha1"
	"github.com/koordinator-sh/koordinator/pkg/runtimeproxy-nri/store"
	"github.com/koordinator-sh/koordinator/pkg/runtimeproxy-nri/utils"
)

type PodResourceExecutor struct {
	store.PodSandboxInfo
}

func NewPodResourceExecutor() *PodResourceExecutor {
	return &PodResourceExecutor{}
}

func (p *PodResourceExecutor) GetMetaInfo() string {
	return ""
}

func (p *PodResourceExecutor) GenerateResourceCheckpoint() interface{} {
	return v1alpha1.ContainerResourceHookRequest{}
}

func (p *PodResourceExecutor) GenerateHookRequest() interface{} {
	return store.ContainerInfo{}
}

func (p *PodResourceExecutor) ParseRequest(request interface{}) (utils.CallHookPluginOperation, error) {
	return utils.Unknown, nil
}
func (p *PodResourceExecutor) ResourceCheckPoint(response interface{}) error {
	return nil
}

func (p *PodResourceExecutor) DeleteCheckpointIfNeed(request interface{}) error {
	return nil
}

func (p *PodResourceExecutor) UpdateRequest(response interface{}, request interface{}) error {
	return nil
}

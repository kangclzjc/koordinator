package resexecutor

import (
	"github.com/koordinator-sh/koordinator/apis/runtime/v1alpha1"
	"github.com/koordinator-sh/koordinator/pkg/runtimeproxy-nri/store"
	"github.com/koordinator-sh/koordinator/pkg/runtimeproxy-nri/utils"
)

type ContainerResourceExecutor struct {
	store.ContainerInfo
}

func NewContainerResourceExecutor() *ContainerResourceExecutor {
	return &ContainerResourceExecutor{}
}

func (c *ContainerResourceExecutor) GetMetaInfo() string {
	return ""
}

func (c *ContainerResourceExecutor) GenerateResourceCheckpoint() interface{} {
	return v1alpha1.ContainerResourceHookRequest{}
}

func (c *ContainerResourceExecutor) GenerateHookRequest() interface{} {
	return store.ContainerInfo{}
}

func (c *ContainerResourceExecutor) ParseRequest(request interface{}) (utils.CallHookPluginOperation, error) {
	return utils.Unknown, nil
}
func (c *ContainerResourceExecutor) ResourceCheckPoint(response interface{}) error {
	return nil
}

func (c *ContainerResourceExecutor) DeleteCheckpointIfNeed(request interface{}) error {
	return nil
}

func (c *ContainerResourceExecutor) UpdateRequest(response interface{}, request interface{}) error {
	return nil
}

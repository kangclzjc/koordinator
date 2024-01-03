/*
Copyright 2022 The Koordinator Authors.

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

package resctrl

import (
	"fmt"
	apiext "github.com/koordinator-sh/koordinator/apis/extension"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/resourceexecutor"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/runtimehooks/hooks"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/runtimehooks/protocol"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/runtimehooks/rule"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/statesinformer"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/util/resctrl"
	rmconfig "github.com/koordinator-sh/koordinator/pkg/runtimeproxy/config"
)

const (
	name        = "Resctrl"
	description = "set resctrl for class/pod"

	ruleNameForNodeSLO  = name + " (nodeSLO)"
	ruleNameForNodeMeta = name + " (nodeMeta)"
	RDT                 = true
)

type plugin struct {
	engine   resctrl.ResctrlEngine
	rule     *Rule
	executor resourceexecutor.ResourceUpdateExecutor
}

func (p *plugin) Register(op hooks.Options) {
	hooks.Register(rmconfig.PreRunPodSandbox, name, description+" (pod)", p.SetPodResCtrlResources)
	hooks.Register(rmconfig.PreCreateContainer, name, description+" (pod)", p.SetContainerResCtrlResources)
	hooks.Register(rmconfig.PreRemoveRunPodSandbox, name, description+" (pod)", p.RemovePodResCtrlResources)
	rule.Register(ruleNameForNodeSLO, description,
		rule.WithParseFunc(statesinformer.RegisterTypeNodeSLOSpec, p.parseRuleForNodeSLO),
		rule.WithUpdateCallback(p.ruleUpdateCbForNodeSLO))
	//reconciler.RegisterCgroupReconciler(reconciler.PodLevel, sysutil.Resctrl, description+" (pod resctl schema)", p.SetPodResCtrlResources, reconciler.PodQOSFilter(), podQOSConditions...)
	//reconciler.RegisterCgroupReconciler(reconciler.ContainerTasks, sysutil.Resctrl, description+" (pod resctl taskids)", p.UpdatePodTaskIds, reconciler.PodQOSFilter(), podQOSConditions...)

	if RDT {
		p.engine = resctrl.NewRDTEngine()
	}
	//else if AMD {
	//    p.engine = AMDEngine{}
	//} else {
	//    p.engine = ARMEngine{}
	//}
	p.engine.Rebuild()
}

func (p *plugin) SetPodResCtrlResources(proto protocol.HooksProtocol) error {
	podCtx := proto.(*protocol.PodContext)
	if podCtx == nil {
		return fmt.Errorf("pod protocol is nil for plugin %v", name)
	}

	return nil
}

func (p *plugin) SetContainerResCtrlResources(proto protocol.HooksProtocol) error {
	containerCtx := proto.(*protocol.ContainerContext)
	if containerCtx == nil {
		return fmt.Errorf("container protocol is nil for plugin %v", name)
	}
	resource := &protocol.Resctrl{
		Schemata: "",
		Hook:     "",
		Closid:   string(apiext.QoSBE),
	}
	containerCtx.Response.Resources.Resctrl = resource
	return nil
}

func (p *plugin) RemovePodResCtrlResources(proto protocol.HooksProtocol) error {
	return nil
}

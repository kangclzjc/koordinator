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
	"os"

	apiext "github.com/koordinator-sh/koordinator/apis/extension"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/resourceexecutor"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/runtimehooks/hooks"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/runtimehooks/protocol"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/runtimehooks/rule"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/statesinformer"
	util "github.com/koordinator-sh/koordinator/pkg/koordlet/util/resctrl"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/util/system"
	rmconfig "github.com/koordinator-sh/koordinator/pkg/runtimeproxy/config"
)

const (
	// LSRResctrlGroup is the name of LSR resctrl group
	LSRResctrlGroup = "LSR"
	// LSResctrlGroup is the name of LS resctrl group
	LSResctrlGroup = "LS"
	// BEResctrlGroup is the name of BE resctrl group
	BEResctrlGroup = "BE"
	// UnknownResctrlGroup is the resctrl group which is unknown to reconcile
	UnknownResctrlGroup = "Unknown"
	name                = "Resctrl"
	description         = "set resctrl for class/pod"

	ruleNameForNodeSLO  = name + " (nodeSLO)"
	ruleNameForNodeMeta = name + " (nodeMeta)"
	RDT                 = true
	ResctrlAnno         = "node.koordinator.sh/resctrl"
)

var (
	// resctrlGroupList is the list of resctrl groups to be reconcile
	resctrlGroupList = []string{LSRResctrlGroup, LSResctrlGroup, BEResctrlGroup}
)

// TODO:@Bowen choose parser there or in engine, should we init with some parameters?
type plugin struct {
	engine   util.ResctrlEngine
	rule     *Rule
	executor resourceexecutor.ResourceUpdateExecutor
}

var singleton *plugin

func Object() *plugin {
	if singleton == nil {
		singleton = newPlugin()
	}
	return singleton
}

func newPlugin() *plugin {
	return &plugin{
		rule: newRule(),
	}
}

func (p *plugin) Register(op hooks.Options) {
	hooks.Register(rmconfig.PreRunPodSandbox, name, description+" (pod)", p.SetPodResctrlResources)
	hooks.Register(rmconfig.PreCreateContainer, name, description+" (pod)", p.SetContainerResctrlResources)
	hooks.Register(rmconfig.PreRemoveRunPodSandbox, name, description+" (pod)", p.RemovePodResctrlResources)
	rule.Register(ruleNameForNodeSLO, description,
		rule.WithParseFunc(statesinformer.RegisterTypeNodeSLOSpec, p.parseRuleForNodeSLO),
		rule.WithUpdateCallback(p.ruleUpdateCbForNodeSLO))
	//reconciler.RegisterCgroupReconciler(reconciler.PodLevel, sysutil.Resctrl, description+" (pod resctl schema)", p.SetPodResCtrlResources, reconciler.PodQOSFilter(), podQOSConditions...)
	//reconciler.RegisterCgroupReconciler(reconciler.ContainerTasks, sysutil.Resctrl, description+" (pod resctl taskids)", p.UpdatePodTaskIds, reconciler.PodQOSFilter(), podQOSConditions...)

	if RDT {
		p.engine = util.NewRDTEngine()
	}
	//else if AMD {
	//    p.engine = AMDEngine{}
	//} else {
	//    p.engine = ARMEngine{}
	//}
	p.engine.Rebuild()
}

func (p *plugin) SetPodResctrlResources(proto protocol.HooksProtocol) error {
	podCtx, ok := proto.(*protocol.PodContext)
	if !ok {
		return fmt.Errorf("pod protocol is nil for plugin %v", name)
	}

	var resctrlInfo *protocol.Resctrl
	if v, ok := podCtx.Request.Annotations[ResctrlAnno]; ok {
		// TODO:@Bowen just save schemata or more info for policy?
		qos := "be" // find qos from cgroup name? better idea?
		resctrlInfo = p.abstractResctrlInfo(podCtx.Request.PodMeta.Name, v, qos)
	}
	err := system.InitCatGroupIfNotExist(resctrlInfo.Closid)
	if err != nil {
		// TODO:@Bowen how to handle create error?
	}

	updater := resourceexecutor.NewResctrlSchemataResource(resctrlInfo.Closid, resctrlInfo.Schemata)
	updater.MergeUpdate()
	podCtx.Response.Resources.Resctrl = resctrlInfo
	return nil
}

func (p *plugin) SetContainerResctrlResources(proto protocol.HooksProtocol) error {
	containerCtx, ok := proto.(*protocol.ContainerContext)
	if !ok {
		return fmt.Errorf("container protocol is nil for plugin %v", name)
	}

	resource := &protocol.Resctrl{
		Schemata: "",
		Hook:     "",
		Closid:   string(apiext.QoSBE),
	}
	containerCtx.Response.Resources.Resctrl = resource
	// add parent pid into right ctrl group

	return nil
}

func (p *plugin) RemovePodResctrlResources(proto protocol.HooksProtocol) error {
	podCtx, ok := proto.(*protocol.PodContext)
	if !ok {
		return fmt.Errorf("pod protocol is nil for plugin %v", name)
	}

	if podCtx.Request.Annotations[ResctrlAnno] != "" {
		if err := os.Remove(system.GetResctrlGroupRootDirPath(podCtx.Request.PodMeta.Name)); err != nil {
			return fmt.Errorf("cannot remove ctrl group, err: %w", err)
		}
	}
	return nil
}

func (p *plugin) abstractResctrlInfo(podId, annotation, qos string) (resource *protocol.Resctrl) {
	if annotation != "" {
		// TODO: convert annotation into schemataRaw? a final schemata discuss in thursday?
		resource = &protocol.Resctrl{
			Schemata: "",
			Hook:     "", // complex, think about how to group it?
			Closid:   podId,
		}
	}

	return resource
}

// func (p *plugin)

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
	"github.com/koordinator-sh/koordinator/pkg/koordlet/util/resctrl"
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
	Anno                = "node.koordinator.sh/resctrl"
)

var (
	// resctrlGroupList is the list of resctrl groups to be reconcile
	resctrlGroupList = []string{LSRResctrlGroup, LSResctrlGroup, BEResctrlGroup}
)

type plugin struct {
	engine   resctrl.ResctrlEngine
	rule     *Rule
	executor resourceexecutor.ResourceUpdateExecutor
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
		p.engine = resctrl.NewRDTEngine()
	}
	//else if AMD {
	//    p.engine = AMDEngine{}
	//} else {
	//    p.engine = ARMEngine{}
	//}
	p.engine.Rebuild()
}

func (p *plugin) SetPodResctrlResources(proto protocol.HooksProtocol) error {
	podCtx := proto.(*protocol.PodContext)
	if podCtx == nil {
		return fmt.Errorf("pod protocol is nil for plugin %v", name)
	}

	var resctrlInfo *protocol.Resctrl
	if v, ok := podCtx.Request.Annotations["nodes.koordinator.sh/resctrl"]; ok {
		// TODO: just save schemata or more info for policy?
		resctrlInfo = p.abstractResctrlInfo(v)
		podCtx.Response.Resources.Resctrl = resctrlInfo
	}
	err := system.InitCtrlGroupIfNotExist(resctrlInfo.Closid)
	if err != nil {
		// TODO: how to handle create error?
	}

	// must called after mount
	ids, _ := system.GetCacheIds()
	resctrlRaw := system.NewResctrlSchemataRaw(ids)
	resctrlRaw.ParseResctrlSchemata(resctrlInfo.Schemata, len(ids))
	groupPath := system.ResctrlSchemata.Path(resctrlInfo.Closid)
	fd, err := os.Open(groupPath)
	defer fd.Close()
	if err != nil {
		// TODO: how to handle fd error?
	}
	_, err = fd.Write([]byte(resctrlRaw.L3String()))
	if err != nil {
		// TODO: how to handle fd error?
	}
	_, err = fd.Write([]byte(resctrlRaw.MBString()))
	if err != nil {
		// TODO: how to handle fd error?
	}
	return nil
}

func (p *plugin) SetContainerResctrlResources(proto protocol.HooksProtocol) error {
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
	// add parent pid into right ctrl group

	return nil
}

func (p *plugin) RemovePodResctrlResources(proto protocol.HooksProtocol) error {
	// TODO: how to handle remove for special pod
	return nil
}

func (p *plugin) abstractResctrlInfo(str string) *protocol.Resctrl {
	// TODO: engine get resctrl info from original string?
	resource := &protocol.Resctrl{
		Schemata: str,
		Hook:     "", // complex, think about how to group it?
		Closid:   string(apiext.QoSBE),
	}
	return resource
}

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
	"github.com/koordinator-sh/koordinator/pkg/koordlet/resourceexecutor"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/runtimehooks/hooks"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/runtimehooks/protocol"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/runtimehooks/rule"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/statesinformer"
	util "github.com/koordinator-sh/koordinator/pkg/koordlet/util/resctrl"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/util/system"
	rmconfig "github.com/koordinator-sh/koordinator/pkg/runtimeproxy/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"os"
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
	engine         util.ResctrlEngine
	rule           *Rule
	executor       resourceexecutor.ResourceUpdateExecutor
	statesInformer statesinformer.StatesInformer
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
	app := p.engine.Rebuild()
	p.executor = op.Executor
	p.statesInformer = op.StatesInformer
	podsMeta := p.statesInformer.GetAllPods()
	currentPods := make(map[string]*corev1.Pod)
	for _, podMeta := range podsMeta {
		pod := podMeta.Pod
		if _, ok := podMeta.Pod.Annotations[ResctrlAnno]; ok {
			group := string(podMeta.Pod.UID)
			currentPods[group] = pod
		}
	}

	for k, v := range app {
		if _, ok := currentPods[k]; !ok {
			if err := os.Remove(system.GetResctrlGroupRootDirPath(v.Closid)); err != nil {
				klog.Errorf("cannot remove ctrl group, err: %w", err)
			}
		}
	}
}

func (p *plugin) SetPodResctrlResources(proto protocol.HooksProtocol) error {
	klog.Infof("=========== SetPodResctrlResources========")

	podCtx, ok := proto.(*protocol.PodContext)
	if !ok {
		return fmt.Errorf("pod protocol is nil for plugin %v", name)
	}

	resctrlInfo := &protocol.Resctrl{}

	if v, ok := podCtx.Request.Annotations[ResctrlAnno]; ok {
		// TODO:@Bowen just save schemata or more info for policy?
		//qos := "be" // find qos from cgroup name? better idea?
		//resctrlInfo = p.abstractResctrlInfo(podCtx.Request.PodMeta.Name, v, qos)
		klog.Infof("=========== get Anno, value is %s", v)
		p.engine.RegisterApp(podCtx.Request.PodMeta.UID, v)

		app, err := p.engine.GetApp(podCtx.Request.PodMeta.UID)
		if err != nil {
			return err
		}
		//updater := resourceexecutor.NewResctrlSchemataResource(podCtx.Request.PodMeta.UID, "MB:0=80;1=80;2=100;3=100")
		klog.Info("----------schemata string is %s", app.Resctrl)
		resctrlInfo.Schemata = app.Resctrl.MBString()
		resctrlInfo.Closid = app.Closid
		//updater := resourceexecutor.NewResctrlSchemataResource(podCtx.Request.PodMeta.UID, schemata)
		//p.executor.Update(true, updater)
		//updater.MergeUpdate()
		podCtx.Response.Resources.Resctrl = resctrlInfo
	}

	return nil
}

func (p *plugin) SetContainerResctrlResources(proto protocol.HooksProtocol) error {
	containerCtx, ok := proto.(*protocol.ContainerContext)
	if !ok {
		return fmt.Errorf("container protocol is nil for plugin %v", name)
	}

	//resource := &protocol.Resctrl{
	//	Schemata: "",
	//	Hook:     "",
	//	Closid:   string(apiext.QoSBE),
	//}
	if _, ok := containerCtx.Request.PodAnnotations[ResctrlAnno]; ok {
		containerCtx.Response.Resources.Resctrl = &protocol.Resctrl{
			Schemata: "",
			Hook:     "",
			Closid:   "koordlet-" + containerCtx.Request.PodMeta.UID,
		}
	}
	// add parent pid into right ctrl group

	return nil
}

func (p *plugin) RemovePodResctrlResources(proto protocol.HooksProtocol) error {
	podCtx, ok := proto.(*protocol.PodContext)
	if !ok {
		return fmt.Errorf("pod protocol is nil for plugin %v", name)
	}

	if podCtx.Request.Annotations[ResctrlAnno] != "" {
		if err := os.Remove(system.GetResctrlGroupRootDirPath("koordlet-" + podCtx.Request.PodMeta.UID)); err != nil {
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

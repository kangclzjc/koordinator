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
	corev1 "k8s.io/api/core/v1"
	"os"

	"k8s.io/klog/v2"

	apiext "github.com/koordinator-sh/koordinator/apis/extension"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/resourceexecutor"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/runtimehooks/hooks"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/runtimehooks/protocol"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/runtimehooks/reconciler"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/statesinformer"
	util "github.com/koordinator-sh/koordinator/pkg/koordlet/util/resctrl"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/util/system"
	sysutil "github.com/koordinator-sh/koordinator/pkg/koordlet/util/system"
	rmconfig "github.com/koordinator-sh/koordinator/pkg/runtimeproxy/config"
)

const (
	name        = "Resctrl"
	description = "set resctrl for class/pod"
	RDT         = true
)

// TODO:@Bowen choose parser there or in engine, should we init with some parameters?
type plugin struct {
	engine         util.ResctrlEngine
	executor       resourceexecutor.ResourceUpdateExecutor
	statesInformer statesinformer.StatesInformer
	app            map[string]util.App
}

var singleton *plugin

func Object() *plugin {
	if singleton == nil {
		singleton = newPlugin()
	}
	return singleton
}

func newPlugin() *plugin {
	return &plugin{}
}

func (p *plugin) init() {
	podsMeta := p.statesInformer.GetAllPods()
	currentPods := make(map[string]*corev1.Pod)
	for _, podMeta := range podsMeta {
		pod := podMeta.Pod
		if _, ok := podMeta.Pod.Annotations[apiext.ResctrlAnno]; ok {
			group := string(podMeta.Pod.UID)
			currentPods[group] = pod
		}
	}

	for k, v := range p.app {
		if _, ok := currentPods[k]; !ok {
			if err := os.Remove(system.GetResctrlGroupRootDirPath(v.Closid)); err != nil {
				klog.Errorf("cannot remove ctrl group, err: %w", err)
			}
		}
	}
}

func (p *plugin) Register(op hooks.Options) {
	if vendorID, err := sysutil.GetVendorIDByCPUInfo(sysutil.GetCPUInfoPath()); err == nil && vendorID == sysutil.INTEL_VENDOR_ID {
		p.engine = util.NewRDTEngine()
	} else {
		//TODO: add AMD resctrl engine
		return
	}

	hooks.Register(rmconfig.PreRunPodSandbox, name, description+" (pod)", p.SetPodResctrlResources)
	hooks.Register(rmconfig.PreCreateContainer, name, description+" (pod)", p.SetContainerResctrlResources)
	hooks.Register(rmconfig.PreRemoveRunPodSandbox, name, description+" (pod)", p.RemovePodResctrlResources)
	reconciler.RegisterCgroupReconciler(reconciler.PodLevel, system.ResctrlSchemata, description+" (pod resctrl schema)", p.SetPodResctrlResources, reconciler.NoneFilter())
	reconciler.RegisterCgroupReconciler(reconciler.PodLevel, system.ResctrlTasks, description+" (pod resctrl tasks)", p.UpdatePodTaskIds, reconciler.NoneFilter())
	reconciler.RegisterCgroupReconciler4AllPods(reconciler.AllPodsLevel, system.ResctrlRoot, description+" (pod resctl taskids)", p.RemoveUnusedResctrlPath, reconciler.PodAnnotationResctrlFilter(), "resctrl")

	p.app = p.engine.Rebuild()
	p.executor = op.Executor
	p.statesInformer = op.StatesInformer
	p.init()
}

func (p *plugin) SetPodResctrlResources(proto protocol.HooksProtocol) error {
	klog.Infof("=========== SetPodResctrlResources========")

	podCtx, ok := proto.(*protocol.PodContext)
	if !ok {
		return fmt.Errorf("pod protocol is nil for plugin %v", name)
	}

	resctrlInfo := &protocol.Resctrl{
		NewTaskIds: make([]int32, 0),
	}

	if v, ok := podCtx.Request.Annotations[apiext.ResctrlAnno]; ok {
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

func (p *plugin) RemoveUnusedResctrlPath(protos []protocol.HooksProtocol) error {
	klog.Infof("=========== RemoveUnusedResctrlPath========")

	currentPods := make(map[string]protocol.HooksProtocol)

	for _, proto := range protos {
		podCtx, ok := proto.(*protocol.PodContext)
		if !ok {
			return fmt.Errorf("pod protocol is nil for plugin %v", name)
		}

		if _, ok := podCtx.Request.Annotations[apiext.ResctrlAnno]; ok {
			group := string(podCtx.Request.PodMeta.UID)
			currentPods[group] = podCtx
		}
		klog.Infof("podCtx is %v", podCtx.Request.Annotations)
	}

	for k, v := range p.app {
		if _, ok := currentPods[k]; !ok {
			if err := os.Remove(system.GetResctrlGroupRootDirPath(v.Closid)); err != nil {
				klog.Errorf("cannot remove ctrl group, err: %w", err)
			}
		}
	}
	return nil
}

func (p *plugin) UpdatePodTaskIds(proto protocol.HooksProtocol) error {
	klog.Infof("=========== UpdatePodTaskIds========")

	podCtx, ok := proto.(*protocol.PodContext)
	if !ok {
		return fmt.Errorf("pod protocol is nil for plugin %v", name)
	}

	if _, ok := podCtx.Request.Annotations[apiext.ResctrlAnno]; ok {
		curTaskMaps := map[string]map[int32]struct{}{}
		var err error
		group := string(podCtx.Request.PodMeta.UID)
		curTaskMaps[group], err = system.ReadResctrlTasksMap(group)
		if err != nil {
			klog.Warningf("failed to read Cat L3 tasks for resctrl group %s, err: %s", group, err)
		}

		newTaskIds := util.GetPodCgroupNewTaskIdsFromPodCtx(podCtx, curTaskMaps[group])
		resctrlInfo := &protocol.Resctrl{
			Closid:     "koordlet-" + group,
			NewTaskIds: make([]int32, 0),
		}
		resctrlInfo.NewTaskIds = newTaskIds

		podCtx.Response.Resources.Resctrl = resctrlInfo

		//resource, err := resourceexecutor.CalculateResctrlL3TasksResource("koordlet-"+group, newTaskIds)
		//if err != nil {
		//	klog.V(4).Infof("failed to get l3 tasks resource for group %s, err: %s", group, err)
		//
		//}
		//updated, err := p.executor.Update(false, resource)
		//if err != nil {
		//	klog.Warningf("failed to write l3 cat policy on tasks for group %s, updated %v, err: %s", group, updated, err)
		//} else if updated {
		//	klog.V(5).Infof("apply l3 cat tasks for group %s finished, updated %v, len(taskIds) %v", group, updated, len(newTaskIds))
		//} else {
		//	klog.V(6).Infof("apply l3 cat tasks for group %s finished, updated %v, len(taskIds) %v", group, updated, len(newTaskIds))
		//}
		//
		//if err != nil {
		//	klog.Warningf("failed to apply l3 cat tasks for group %s, err %s", group, err)
		//}
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
	if _, ok := containerCtx.Request.PodAnnotations[apiext.ResctrlAnno]; ok {
		containerCtx.Response.Resources.Resctrl = &protocol.Resctrl{
			Schemata:   "",
			Hook:       "",
			Closid:     "koordlet-" + containerCtx.Request.PodMeta.UID,
			NewTaskIds: make([]int32, 0),
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

	if podCtx.Request.Annotations[apiext.ResctrlAnno] != "" {
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
			Schemata:   "",
			Hook:       "", // complex, think about how to group it?
			Closid:     podId,
			NewTaskIds: make([]int32, 0),
		}
	}

	return resource
}

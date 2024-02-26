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
	"encoding/json"
	"fmt"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/resourceexecutor"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/runtimehooks/hooks"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/runtimehooks/protocol"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/runtimehooks/rule"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/statesinformer"
	util "github.com/koordinator-sh/koordinator/pkg/koordlet/util/resctrl"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/util/system"
	sysutil "github.com/koordinator-sh/koordinator/pkg/koordlet/util/system"
	rmconfig "github.com/koordinator-sh/koordinator/pkg/runtimeproxy/config"
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

type ResctrlConfig struct {
	LLC LLC `json:"LLC,omitempty"`
	MB  MB  `json:"MB,omitempty"`
}

type LLC struct {
	Schemata         SchemataConfig           `json:"schemata,omitempty"`
	SchemataPerCache []SchemataPerCacheConfig `json:"schemataPerCache,omitempty"`
}

type MB struct {
	Schemata         SchemataConfig           `json:"schemata,omitempty"`
	SchemataPerCache []SchemataPerCacheConfig `json:"schemataPerCache,omitempty"`
}

type SchemataConfig struct {
	Percent int   `json:"percent,omitempty"`
	Range   []int `json:"range,omitempty"`
}

type SchemataPerCacheConfig struct {
	CacheID        int `json:"cacheid,omitempty"`
	SchemataConfig `json:",inline"`
}

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
	p.executor = op.Executor
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

		// Parse the JSON value into the BlockIO struct
		var res ResctrlConfig
		err := json.Unmarshal([]byte(v), &res)
		if err != nil {
			klog.Errorf("error is %v", err)
			//panic(err)
			return nil
		}

		// Print the parsed data
		klog.Infof("resctrl: %v", res)
		if res.MB.Schemata.Percent != 0 && res.MB.Schemata.Range != nil {
			klog.Infof("resctrl MB is : %v", res.MB)
		}
		//err = system.InitCatGroupIfNotExist(podCtx.Request.PodMeta.UID)
		//if err != nil {
		//	// TODO:@Bowen how to handle create error?
		//	klog.Errorf("error is %v", err)
		//}

		schemata := ParseSchemata(res)

		//updater := resourceexecutor.NewResctrlSchemataResource(podCtx.Request.PodMeta.UID, "MB:0=80;1=80;2=100;3=100")
		klog.Info("----------schemata string is %s", schemata)
		resctrlInfo.Schemata = schemata
		resctrlInfo.Closid = "koordlet-" + podCtx.Request.PodMeta.UID
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
		if err := os.Remove(system.GetResctrlGroupRootDirPath(podCtx.Request.PodMeta.UID)); err != nil {
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

func ParseSchemata(config ResctrlConfig) string {
	ids, _ := sysutil.CacheIdsCacheFunc()
	schemataRaw := sysutil.NewResctrlSchemataRaw(ids).WithL3Num(len(ids))
	if config.MB.Schemata.Percent != 0 {
		percent := calculateIntel(int64(config.MB.Schemata.Percent))
		for k, _ := range schemataRaw.MB {
			schemataRaw.MB[k] = percent
		}
	}

	if config.MB.SchemataPerCache != nil {
		for _, v := range config.MB.SchemataPerCache {
			percent := calculateIntel(int64(v.Percent))
			schemataRaw.MB[v.CacheID] = percent
		}
	}
	return schemataRaw.MBString()
}

func calculateIntel(mbaPercent int64) int64 {
	if mbaPercent%10 != 0 {
		actualPercent := mbaPercent/10*10 + 10
		klog.V(4).Infof("cat MBA must multiple of 10, mbaPercentConfig is %d, actualMBAPercent will be %d",
			mbaPercent, actualPercent)
		return actualPercent
	}

	return mbaPercent
}

// func (p *plugin)

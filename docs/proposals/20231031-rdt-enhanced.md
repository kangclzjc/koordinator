---
title: Resctrl Runtime Hook
authors:
  - "@kangclzjc"
  - "@bowen-intel"
reviewers:
  - "@zwzhang0107"
  - "@saintube"
  - "@hormes"
creation-date: 2023-11-01
last-updated: 2023-11-02
---

# RDT enhanced

## Table of Contents

Table of Contents
=================

* [RDT enhanced](#rdt-enhanced)
  * [Table of Contents](#table-of-contents)
  * [Glossary](#glossary)
  * [Summary](#summary)
  * [Motivation](#motivation)
    * [Goals](#goals)
    * [Non-Goals/Future Work](#non-goalsfuture-work)
  * [Proposal](#proposal)
    * [User Stories](#user-stories)
      * [Story 1](#story-1)
      * [Story 2](#story-2)
      * [Story 3](#story-3)
    * [Requirements (Optional)](#requirements-optional)
      * [Functional Requirements](#functional-requirements)
        * [FR1](#fr1)
        * [FR2](#fr2)
      * [Non-Functional Requirements](#non-functional-requirements)
        * [NFR1](#nfr1)
        * [NFR2](#nfr2)
    * [Implementation Details/Notes/Constraints](#implementation-detailsnotesconstraints)
    * [Risks and Mitigations](#risks-and-mitigations)
  * [Alternatives](#alternatives)
  * [Upgrade Strategy](#upgrade-strategy)
  * [Additional Details](#additional-details)
    * [Test Plan [optional]](#test-plan-optional)
  * [Implementation History](#implementation-history)

## Glossary


Resource Control (resctrl) is a kernel interface for CPU resource allocation The resctrl interface is available in kernels 4.10 and newer. Currently, Resource Control supports L2 CAT, L3 CAT and L3 CDP which allows partitioning L2 and L3 cache on a per core/task basis. It also supports MBA, the maximum bandwidth can be specified in percentage or in megabytes per second (with an optional mba_MBps flag).

Intel refers to this feature as Intel Resource Director Technology(Intel(R) RDT). AMD refers to this feature as AMD Platform Quality of Service(AMD QoS).

Intel® Resource Director Technology (Intel® RDT) brings new levels of visibility and control over how shared resources such as last-level cache (LLC) and memory bandwidth are used by applications, virtual machines (VMs), and containers. See: https://www.intel.com/content/www/us/en/architecture-and-technology/resource-director-technology.html

AMD QoS are intended to provide for the monitoring of the usage of certain system resources by one or more processors and for the separate allocation and enforcement of limits on the use of certain system resources by one or more processors. The initial QoS functionality is for L3 cache allocation enforcement, L3 cache occupancy monitoring, L3 code-data prioritization, and memory bandwidth enforcement/allocation. See: https://www.amd.com/content/dam/amd/en/documents/processor-tech-docs/other/56375_1_03_PUB.pdf


## Summary

We hope to enhance LLC and memory bandwidth usage in Koordinator to leverage NRI to bind pod to resctrl control group, integrate LLC and memory bandwidth monitor feature and add pod level LLC and memory bandwidth control in realtime into Koordinator.

## Motivation

Koordinator support LLC and memory bandwidth configuration and adjustment by config map based on class level. It uses a goroutine to set/adjust RDT configuration in async mode which may not in real time. As Koordinator already support NRI in 0.3.0 release, we can migrate current function into Koordlet runtimehooks as an runtime hook plugin which could be more real time. Also we want to enhance resctrl at the same time which include monitor and pod level RDT configure/adjustment. With these features, Koordinator can have more information to determine how to adjust different workloads' LLC and memory bandwidth or even other resources' configuration.

### Goals

- Migrate existed fixed class LLC and memory bandwidth function into runtime hook based on NRI
- Add LLC and memory bandwidth monitor for class level
- Add pod level LLC and memory bandwidth configuration/adjustment and monitor

### Non-Goals/Future Work

- ResCtrl policy to better use LLC and memory bandwidth resource
- QoS manager plugin to detect noisy neighbor based on CPU, Memory, LLC and memory bandwidth ... to determine RDT adjustment
- Scheduler based on LLC and memory bandwidth resource

## Proposal

We will implement LLC and memory bandwidth related function as a runtime hook plugin. RDT runtime hook plugin will still watch a configmap and generate UpdateContainer CRI request to dynamically adjust RDT configuration. In the meanwhile, RDT runtime hook plugin will create RDT group and monitor group first for each class during Pod RunPodSandBox and modify OCI spec for pod and container. We rely on runc to bind container/pod to specific control group and monitor group. A. We will collect LLC and memory bandwidth metrics and save them to DB.

### User Stories

#### Story 1
As a cluster administer, I want to apply and adjust L3/MBA QoS class configuration during runtime.
#### Story 2
As a user, I want to adjust my workload's L3/MBA resource during runtime.
#### Story 3
As a cluster administer, I want to monitor cluster L3/MBA resource usage.

### Requirements (Optional)
Need Koordinator to upgrade to 1.3.0+

#### Functional Requirements

Resctrl runtime hook plugin should support all existing functionalities by current Resctrl QoS plugin

##### FR1

##### FR2

#### Non-Functional Requirements

Non-functional requirements are user expectations of the solution. Include
considerations for performance, reliability and security.

##### NFR1

##### NFR2



### Implementation Details/Notes/Constraints
Implement a resctrl runtime hook plugin and a resctrl metrics advisor collector to collect and monitor LLC and MBA resource.

We will have several steps in this proposal:
1. Migrate current QoS class level resctrl function
2. Add monitor for QoS class level resctrl metrics advisor collector
3. Add pod level resctrl support for both control and monitor

Below is implementation detail:

runtime hook plugin
Init:
1. when plugin init, registe rule to create QoS ctrl group based on NodeSLO config, and it will automatically create a monitor group
2. For group level LLC and MBA config, use rule update LLC and MBA config


For pod with specific  request, we will use "node.koordinator.sh/resctrl": "{l3:1=80%, mba:1=80%}"
1. Subscribe RunPodSanbox, when pod with the annotation，ResctrlEngine will parse LLC and MBA configuration based on the annotation and put the result to PodContext.
2. Pod Context will create an extra ctrl group and monitor group for the pod
3. Subscribe CreateContainer, resctrl runtime hook will get closid and runc prestart hook from ResctrlEngine
4. Subscribe RemovePodSandBox, resctrl runtime hook will remove corresponding control group and monitor group


For pod without specific  request:
1. Subscribe RunPodSanbox, if pod without annotation like "node.koordinator.sh/resctrl": "{l3:1=80%, mba:1=80%}", RDTEngine will just record this pod
2. Subscribe CreateContainer, resctrl runtime hook will get closid and runc prestart hook from ResctrlEngine


Resctrl Runtime Hook

```go
type plugin struct {
	engine ResctrlEngine
	rule     *Rule
	executor resourceexecutor.ResourceUpdateExecutor
}

func (p *plugin) Register(op hooks.Options) {
    hooks.Register(rmconfig.PreRunPodSandbox, name, description+" (pod)", p.SetPodResCtrlResources)
    hooks.Register(rmconfig.CreateContainer, name, description+" (pod)", p.SetContainerResCtrlResources)
    hooks.Register(rmconfig.UpdateContainer, name, description+" (pod)", p.UpdateContainerResCtrlResources)
    hooks.Register(rmconfig.RemoveRunPodSandbox, name, description+" (pod)", p.RemovePodResCtrlResources)
    rule.Register(ruleNameForNodeSLO, description,
      rule.WithParseFunc(statesinformer.RegisterTypeNodeSLOSpec, p.parseRuleForNodeSLO),
      rule.WithUpdateCallback(p.ruleUpdateCbForNodeSLO))
	reconciler.RegisterCgroupReconciler(reconciler.PodLevel, sysutil.Resctrl, description+" (pod resctl schema)", p.SetPodResCtrlResources, reconciler.PodQOSFilter(), podQOSConditions...)
    reconciler.RegisterCgroupReconciler(reconciler.ContainerTasks, sysutil.Resctrl, description+" (pod resctl taskids)", p.SetPodResCtrlResources, reconciler.PodQOSFilter(), podQOSConditions...)

if RDT {
        p.engine = NewRDTEngine()
    }
	else if AMD {
        p.engine = AMDEngine{}
    } else {
        p.engine = ARMEngine{}
    }
}

// parseRuleForNodeSLO will parse Resctrl rule from NodeSLO
func (p *plugin) parseRuleForNodeSLO() {

}

// ruleUpdateCbForNodeSLO will update RDT QoS class schemata in resctrl filesystem
func (p *plugin) ruleUpdateCbForNodeSLO() {
	// Get config from NodeSLO
    p.engine.Config(config)
    
    for class := range (classes) {
		e := audit.V(3).Group("RDT").Reason(name).Message("set %s to %v", class, schemata)
		updater, err := resourceexecutor.DefaultCgroupUpdaterFactory.New(sysutil.Resctrl, cgroupPath, schemata, e)
		    p.executor.Update(cgroup, schemata)...
	}
}
	
	

// SetPodResctrl will set control group and monitor group info based on annotation to PodContext
func (p *plugin) SetPodResCtrlResources(proto protocol.HooksProtocol) error {
    resctrl, err := engine.RegisterApp(podid, PodMeta)
	updatePodContext(podid, resctrl)
}

// RemovePodResCtrlResources will set Resctrl remove msg to PodContext
func (p *plugin) RemovePodResCtrlResources(proto protocol.HooksProtocol) error {
    resctrl, err := engine.RegisterApp(podid, RDTMeta)
    updatePodContext(podid, resctrl)
}

// SetContainerResCtrlResources will get Resctrl meta data and update ContainerContext
func (p *plugin) SetContainerResCtrlResources(proto protocol.HooksProtocol) error {
    resctrl, err := engine.GetResCtrl(podid)
	updateContainerContext(podid, containerid, resctrl)
}

// UpdateContainerResCtrlResources will get Resctrl meta data and update ContainerContext
func (p *plugin) UpdateContainerResCtrlResources(proto protocol.HooksProtocol) error {
    resctrl, err := engine.GetResCtrl(podMeta)
    updateContainerContext(podid, containerid, resctrl)
}

```

PodContext
```go
func injectResctrl(cgroupParent string, schemeta string, remove bool, a *audit.EventHelper, e resourceexecutor.ResourceUpdateExecutor) (resourceexecutor.ResourceUpdater, error) {
	// update, err := ...
	// for specific pods, create control group and monitor group and update schemata
	// if it is remove and is specific pods, updater need to remove this control group and monitor group
}
```

ContainerContext
```go
func (c *ContainerContext) NriDone(executor resourceexecutor.ResourceUpdateExecutor) (*api.ContainerAdjustment, *api.ContainerUpdate, error) {
    ...
	if c.Response.Resources.RDT != nil {
        // adjust OCI spec
		adjust.SetLinuxRDTClass(c.Response.Resources.RDT.class)
	}
}
```

ResctrlUpdater
We will enhance current ResctrlUpdate to support directly overwrite schemata. Current ResctrlUpdater doesn't support override schemata directly and doesn't consider NUMA when apply schemata .

```go
func NewResctrSchemata(group, schemata string) ResourceUpdater {

} 

```

PodTaskIdsStatesInformer
PodTaskIdsStatesInformer will get all pods' taskids, Resctl runtime hook Reconciler will register callback to consume these information and write new taskIds to resctrl contrl group and monitor group.

Reconciler
Reconciler will help guarantee eventual consistency of Resctrl configuration. It will reconcile all QoS class resctrl config based on NodeSLO. For pod level, it will reconcile all pods resctrl config based on their annotations. Performance issue, enhance.
```go
func (c *reconciler) reconcileResCtrlGroupAndPolicy(stopCh <-chan struct{}) {
	for {
		// 1. reconcile NodeSLO
		// 2. reconcile Pods taskids and check specific pods control group and monitor group is existed if not help do it, and if pod has been removed, it will also help to remove ctrl group and monitor group
		time.Sleep(time.Second * 1)
	}
}
```

ResCtrlEngine
// nodes.koordinator.sh/resctrl: `{l3:1=90,2=90, MBA:70}`
```go
type ResctrlEngine interface {
    RegisterApp(podid, value) (ResCtrl, error)
    GetResCtrl(podid) (ResCtrl, error)
    //GetResCtrlFromPod(value) (ResCtrl, error)
}

type Resctrl interface {
    GetL3() string
    GetMBA() string
}
```

For Different platform, we will implement different ResctrlEngine like RDTEngine for intel, AMDEngine for AMD, ARMEngine for ARM.
RDTEngine will implement ResCtrlEngine interface. Currently, RDTEngine is very simple and only focus on parse pod RDT resource request and resctrl runtime hook will update ContainerContext based on these info. In the future, we will have policy in engine for dynamically adjust resctrl resources.

Resctrl Monitor
Currently, we only retrieve QoS class level resctrl data like LLC and MBA. For further pod level monitor data, we will iterate all pods and get pod level metric
1. collector check whether resctrl file system supported and mounted
2. iterate all QoS class monitor group in resctrl file system and read data from it and save data to DB
4. iterate all Pods monitor group in resctrl file system and read data from it and save data to DB
```go
func (p *ResourceCtrlCollector) collectQoSClassResctrlResUsed() {
    // NodeSLO, get all class level Resctrl usage
	for class := range classes {
        resctrl, err := GetResctrlUsage(classCgroupPath)
    }
}

func (p *ResourceCtrlCollector) collectQoSClassResctrlResUsed() {
	// Pods, get all specific pod Resctrl usage
    for pod := range pods {
        resctrl, err := GetResctrlUsage(podCgroupPath)
    }
}
```

### Risks and Mitigations

- What are the risks of this proposal and how do we mitigate? Think broadly.
- How will UX be reviewed and by whom?
- How will security be reviewed and by whom?
- Consider including folks that also work outside the SIG or subproject.

## Alternatives

RDT QoSManager Plugin is an asynchronize plugin which may not reconcile RDT resource in real time and need to iterate all task ids in pod/container periodically.

## Upgrade Strategy

## Additional Details

### Test Plan [optional]

## Implementation History

- [ ] MM/DD/YYYY: Proposed idea in an issue or [community meeting]
- [ ] MM/DD/YYYY: Compile a Google Doc following the CAEP template (link here)
- [ ] MM/DD/YYYY: First round of feedback from community
- [ ] MM/DD/YYYY: Present proposal at a [community meeting]
- [ ] MM/DD/YYYY: Open proposal PR



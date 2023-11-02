---
title: NRI mode Resource Management
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

A table of contents is helpful for quickly jumping to sections of a proposal and for highlighting
any additional information provided beyond the standard proposal template.
[Tools for generating](https://github.com/ekalinin/github-markdown-toc) a table of contents from markdown are available.

- [Title](#title)
  - [Table of Contents](#table-of-contents)
  - [Glossary](#glossary)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals/Future Work](#non-goalsfuture-work)
  - [Proposal](#proposal)
    - [User Stories](#user-stories)
      - [Story 1](#story-1)
      - [Story 2](#story-2)
    - [Requirements (Optional)](#requirements-optional)
      - [Functional Requirements](#functional-requirements)
        - [FR1](#fr1)
        - [FR2](#fr2)
      - [Non-Functional Requirements](#non-functional-requirements)
        - [NFR1](#nfr1)
        - [NFR2](#nfr2)
    - [Implementation Details/Notes/Constraints](#implementation-detailsnotesconstraints)
    - [Risks and Mitigations](#risks-and-mitigations)
  - [Alternatives](#alternatives)
  - [Upgrade Strategy](#upgrade-strategy)
  - [Additional Details](#additional-details)
    - [Test Plan [optional]](#test-plan-optional)
  - [Implementation History](#implementation-history)

## Glossary

RDT, Resource Directory Technology. See: https://www.intel.com/content/www/us/en/architecture-and-technology/resource-director-technology.html

## Summary

We hope to enhance RDT usage in koordinator to import pod level RDT adjustment in realtime and integrate RDT monitor feature into koordinator.

## Motivation

Koordinator support RDT configuration and adjustment by config map based on class level. It uses a goroutine to set/adjust RDT configuration in async mode which may not in real time. As koordinator already support NRI in 0.3.0 release, we can migrate current function into koordlet as an runtime hook plugin which may be more real time. Also we want to enhance RDT feature in the same time which include pod level RDT configure/adjustment and monitor. With these features, koordinator can have more information to determine how to adjust different workloads' RDT or even other resources' configuration.

### Goals

- Migrate existed RDT function into runtime hook based on NRI
- Add pod level RDT configuration/adjustment
- Add monitor for class and pod level

### Non-Goals/Future Work

- RDT configure policy
- Schedule workload based on RDT resource
- AMD ResCtl support

## Proposal

We will implement RDT related function as a runtime hook plugin. RDT runtime hook plugin will still watch a configmap and generate UpdateContainer CRI request to dynamically adjust RDT configuration. In the meanwhile, RDT runtime hook plugin will create RDT group and monitor group first for each class during Pod RunPodSandBox and modify OCI spec for pod and container. We rely on runc to make RDT configuration work. A. We will collect RDT resource metrics and report them to koordlet.

### User Stories

#### Story 1
As a cluster administer, I want to apply and adjust RDT QoS class configuration during runtime. 
#### Story 2
As a user, I want to guarantee and adjust my workload's RDT resource during runtime.
#### Story 3
As a cluster administer, I want to monitor cluster RDT resource usage.

### Requirements (Optional)
Need Koordinator to upgrade to 1.3.0+

#### Functional Requirements

RDT runtime hook plugin should support all existing functionalities by current RDT plugin

##### FR1

##### FR2

#### Non-Functional Requirements

Non-functional requirements are user expectations of the solution. Include
considerations for performance, reliability and security.

##### NFR1

##### NFR2

### Implementation Details/Notes/Constraints
Implement a RDT runtime hook plugin. And we implement a RDT metrics advisor collector to collect and monitor RDT resource.

runtime hook plugin 

1. when plugin init, create QoS ctrl group, and it will automatically create a monitor group
2. Subscribe RunPodSanbox, when pod with annotation like rdt.intel.com/l3=80%，give an extra ctrl group
3. Subscribe CreateContainer, when pod withtout annotation, generate RDT oci spec, use runc to apply to resctl file system // open: will runc help create a RDT group
4. For group level RDT config, use rule to update RDT config
5. For pod with annotation like rdt.intel.com/l3=80%, Subscribe UpdateContainer, when update container, we will update container RDT // open: who will update this
6. 
```go
type plugin struct {
  rule     *Rule
  executor resourceexecutor.ResourceUpdateExecutor
}

func (p *plugin) Register(op hooks.Options) {
	
}

func (p *plugin) CreateRDTgroup(proto protocol.HooksProtocol) error {
    
}

func (p *plugin) CreateRDTMonitorgroup(proto protocol.HooksProtocol) error {

}

func (p *plugin) SetPodRDT(proto protocol.HooksProtocol) error {
	
}

func (p *plugin) SetContainerRDT(proto protocol.HooksProtocol) error { 
	
}

func (p *plugin) UpdateContainerRDT() error {
	// generate a UpdateContainer CRI request.
}

```

RDT collector
1. collector check whether resctrl file system supported and mounted
2. iterate all monitor group, and get class, pod -> metrics
3. LS/PodID_XXX, BE
```go

```

### Risks and Mitigations

- What are the risks of this proposal and how do we mitigate? Think broadly.
- How will UX be reviewed and by whom?
- How will security be reviewed and by whom?
- Consider including folks that also work outside the SIG or subproject.

## Alternatives

RDT QoSManager Plugin is an asynchronize plugin which may not reconcile RDT resource in real time and need to iterate all task ids in pod/container periodically.

## Upgrade Strategy
[cgroup_manager_linux_test.go](..%2F..%2F..%2F..%2Fgithub.com%2Fkubernetes%2Fpkg%2Fkubelet%2Fcm%2Fcgroup_manager_linux_test.go)
If applicable, how will the component be upgraded? Make sure this is in the test plan.

Consider the following in developing an upgrade strategy for this enhancement:
- What changes (in invocations, configurations, API use, etc.) is an existing cluster required to make on upgrade in order to keep previous behavior?
- What changes (in invocations, configurations, API use, etc.) is an existing cluster required to make on upgrade in order to make use of the enhancement?

## Additional Details

### Test Plan [optional]

## Implementation History

- [ ] MM/DD/YYYY: Proposed idea in an issue or [community meeting]
- [ ] MM/DD/YYYY: Compile a Google Doc following the CAEP template (link here)
- [ ] MM/DD/YYYY: First round of feedback from community
- [ ] MM/DD/YYYY: Present proposal at a [community meeting]
- [ ] MM/DD/YYYY: Open proposal PR


---
title: NRI mode Resource Management
authors:
  - "@kangclzjc"
  - "@whu16"
  - "@hle2"
reviewers:
  - "@zwzhang0107"
  - "@saintube"
  - "@hormes"
creation-date: 2023-06-08
last-updated: 2023-06-15
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

## Proposal

We will implement RDT related function as a runtime hook plugin. It will still watch ConfigMap.

[PlantUML](http://plantuml.com) is the preferred tool to generate diagrams,
place your `.plantuml` files under `images/` and run `make diagrams` from the docs folder.

### User Stories

- Detail the things that people will be able to do if this proposal is implemented.
- Include as much detail as possible so that people can understand the "how" of the system.
- The goal here is to make this feel real for users without getting bogged down.

#### Story 1

#### Story 2

### Requirements (Optional)

Some authors may wish to use requirements in addition to user stories.
Technical requirements should be derived from user stories, and provide a trace from
use case to design, implementation and test case. Requirements can be prioritised
using the MoSCoW (MUST, SHOULD, COULD, WON'T) criteria.

The difference between goals and requirements is that between an executive summary
and the body of a document. Each requirement should be in support of a goal,
but narrowly scoped in a way that is verifiable or ideally - testable.

#### Functional Requirements

Functional requirements are the properties that this design should include.

##### FR1

##### FR2

#### Non-Functional Requirements

Non-functional requirements are user expectations of the solution. Include
considerations for performance, reliability and security.

##### NFR1

##### NFR2

### Implementation Details/Notes/Constraints

- What are some important details that didn't come across above.
- What are the caveats to the implementation?
- Go in to as much detail as necessary here.
- Talk about core concepts and how they releate.

### Risks and Mitigations

- What are the risks of this proposal and how do we mitigate? Think broadly.
- How will UX be reviewed and by whom?
- How will security be reviewed and by whom?
- Consider including folks that also work outside the SIG or subproject.

## Alternatives

The `Alternatives` section is used to highlight and record other possible approaches to delivering the value proposed by a proposal.

## Upgrade Strategy

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


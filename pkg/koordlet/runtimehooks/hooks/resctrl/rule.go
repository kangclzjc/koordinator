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
	"sync"

	slov1alpha1 "github.com/koordinator-sh/koordinator/apis/slo/v1alpha1"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/statesinformer"
	"k8s.io/klog/v2"
)

type Rule struct {
	lock sync.RWMutex
}

func newRule() *Rule {
	return &Rule{}
}

func (p *plugin) parseRuleForNodeSLO(mergedNodeSLOIf interface{}) (bool, error) {
	mergedNodeSLO := mergedNodeSLOIf.(*slov1alpha1.NodeSLOSpec)
	if mergedNodeSLO == nil || mergedNodeSLO.ResourceQOSStrategy == nil {
		// do nothing if nodeSLO == nil || nodeSLO.spec.ResourceStrategy == nil
		klog.Warningf("nodeSLO is nil %v, or nodeSLO.Spec.ResourceQOSStrategy is nil", mergedNodeSLO == nil)
		return false, nil
	}

	// calculate and apply l3 cat policy for each group
	for _, group := range resctrlGroupList {
		resQoSStrategy := getResourceQOSForResctrlGroup(mergedNodeSLO.ResourceQOSStrategy, group)
		if resQoSStrategy == nil || resQoSStrategy.ResctrlQOS == nil || resQoSStrategy.ResctrlQOS.CATRangeStartPercent == nil ||
			resQoSStrategy.ResctrlQOS.CATRangeEndPercent == nil {
			klog.Warningf("skipped, since resourceQoS or startPercent or endPercent is nil for group %v, "+
				"resourceQoS %v", resQoSStrategy, group)
			return false, nil
		}

		startPercent, endPercent := *resQoSStrategy.ResctrlQOS.CATRangeStartPercent, *resQoSStrategy.ResctrlQOS.CATRangeEndPercent
		p.engine.Config(string(startPercent) + ":" + string(endPercent))

		// calculate updating resource
		// resource := resourceexecutor.NewResctrlL3SchemataResource(group, l3MaskValue, l3Num)
	}

	return false, nil
}

func (p *plugin) ruleUpdateCbForNodeSLO(target *statesinformer.CallbackTarget) error {
	return nil
}

func getResourceQOSForResctrlGroup(strategy *slov1alpha1.ResourceQOSStrategy, group string) *slov1alpha1.ResourceQOS {
	if strategy == nil {
		return nil
	}
	switch group {
	case LSRResctrlGroup:
		return strategy.LSRClass
	case LSResctrlGroup:
		return strategy.LSClass
	case BEResctrlGroup:
		return strategy.BEClass
	}
	return nil
}

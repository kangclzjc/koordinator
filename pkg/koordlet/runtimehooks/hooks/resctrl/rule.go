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
	"github.com/koordinator-sh/koordinator/pkg/koordlet/statesinformer"
	"sync"
)

type Rule struct {
	lock sync.RWMutex
}

func (p *plugin) parseRuleForNodeSLO(mergedNodeSLOIf interface{}) (bool, error) {
	return false, nil
}

func (p *plugin) ruleUpdateCbForNodeSLO(target *statesinformer.CallbackTarget) error {
	return nil
}

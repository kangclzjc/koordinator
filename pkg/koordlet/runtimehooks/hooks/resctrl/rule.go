package resctrl

import (
	apiext "github.com/koordinator-sh/koordinator/apis/extension"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/statesinformer"
	util "github.com/koordinator-sh/koordinator/pkg/koordlet/util/resctrl"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/util/system"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"os"
	"strings"
	"sync"
)

type Rule struct {
	lock sync.RWMutex
}

func newRule() *Rule {
	return &Rule{}
}

func (p *plugin) ruleUpdateCbForAllPods(target *statesinformer.CallbackTarget) error {
	if target == nil {
		klog.Warningf("callback target is nil")
		return nil
	}

	// NOTE: if the ratio becomes bigger, scale top down, otherwise, scale bottom up
	if p.rule == nil {
		klog.V(5).Infof("hook plugin rule is nil, nothing to do for plugin %v", ruleNameForAllPods)
		return nil
	}

	p.engine.Rebuild()
	apps := p.engine.GetApps()

	currentPods := make(map[string]*corev1.Pod)
	for _, podMeta := range target.Pods {
		pod := podMeta.Pod
		if _, ok := podMeta.Pod.Annotations[apiext.AnnotationResctrl]; ok {
			group := string(podMeta.Pod.UID)
			currentPods[group] = pod
		}
	}

	for k, v := range apps {
		if _, ok := currentPods[k]; !ok {
			if err := os.Remove(system.GetResctrlGroupRootDirPath(v.Closid)); err != nil {
				klog.Errorf("cannot remove ctrl group, err: %w", err)
				if os.IsNotExist(err) {
					p.engine.UnRegisterApp(strings.TrimPrefix(v.Closid, util.ClosdIdPrefix))
				}
			} else {
				p.engine.UnRegisterApp(strings.TrimPrefix(v.Closid, util.ClosdIdPrefix))
			}
		}
	}
	return nil
}

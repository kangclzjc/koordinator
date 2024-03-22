package util

import (
	"encoding/json"
	"fmt"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/runtimehooks/protocol"
	koordletutil "github.com/koordinator-sh/koordinator/pkg/koordlet/util"
	sysutil "github.com/koordinator-sh/koordinator/pkg/koordlet/util/system"
	"io/ioutil"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const ClosdIdPrefix = "koordlet-"

type Resctrl struct {
	L3 map[int]string
	MB map[int]string
}

type App struct {
	Resctrl *sysutil.ResctrlSchemataRaw
	// Hooks   Hook
	Closid string
}

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

// TODO: @Bowen we should talk about this interface functions' meaning?
type ResctrlEngine interface {
	Rebuild() // rebuild the current control group
	RegisterApp(podid, annotation string) error
	UnRegisterApp(podid string) error
	GetApp(podid string) (App, error)
	GetApps() map[string]App
}

func NewRDTEngine() ResctrlEngine {
	return &RDTEngine{
		Apps:       make(map[string]App),
		CtrlGroups: make(map[string]Resctrl),
	}
}

type RDTEngine struct {
	Apps       map[string]App
	CtrlGroups map[string]Resctrl
	l          sync.RWMutex
}

func (R *RDTEngine) UnRegisterApp(podid string) error {
	if _, ok := R.Apps[podid]; !ok {
		return fmt.Errorf("pod %s not registered", podid)
	}
	R.l.Lock()
	defer R.l.Unlock()
	delete(R.Apps, podid)
	return nil
}

func (R *RDTEngine) GetApps() map[string]App {
	R.l.RLock()
	defer R.l.RUnlock()
	apps := make(map[string]App)
	for podid, app := range R.Apps {
		apps[podid] = app
	}
	return apps
}

func (R *RDTEngine) Rebuild() {
	// get resctrl filesystem root
	root := sysutil.GetResctrlSubsystemDirPath()

	files, err := os.ReadDir(root)
	if err != nil {
		klog.Errorf("read %s failed err is %v", root, err)
		return
	}

	for _, file := range files {
		if file.IsDir() && strings.HasPrefix(file.Name(), ClosdIdPrefix) {
			path := filepath.Join(root, file.Name(), "schemata")
			if _, err := os.Stat(path); err == nil {
				content, err := ioutil.ReadFile(path)
				if err != nil {
					klog.Errorf("read resctrl file path fail, %v", err)
					return
				}
				schemata := string(content)
				ids, _ := sysutil.CacheIdsCacheFunc()
				schemataRaw := sysutil.NewResctrlSchemataRaw(ids).WithL3Num(len(ids))
				err = schemataRaw.ParseResctrlSchemata(schemata, -1)
				if err != nil {
					klog.Errorf("failed to parse %v", err)
				}
				podid := strings.TrimPrefix(file.Name(), ClosdIdPrefix)
				R.l.Lock()
				defer R.l.Unlock()
				R.Apps[podid] = App{
					Resctrl: schemataRaw,
					Closid:  file.Name(),
				}
			}
		}
	}
}

func (R *RDTEngine) RegisterApp(podid, annotation string) error {
	if _, ok := R.Apps[podid]; ok {
		return fmt.Errorf("pod %s already registered", podid)
	}
	// Parse the JSON value into the BlockIO struct
	var res ResctrlConfig
	err := json.Unmarshal([]byte(annotation), &res)
	if err != nil {
		klog.Errorf("error is %v", err)
		return err
	}

	schemata := ParseSchemata(res)
	app := App{
		Resctrl: schemata,
		Closid:  ClosdIdPrefix + podid,
	}
	R.l.Lock()
	defer R.l.Unlock()
	R.Apps[podid] = app
	return nil
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

func ParseSchemata(config ResctrlConfig) *sysutil.ResctrlSchemataRaw {
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
	return schemataRaw
}

func (R *RDTEngine) GetApp(id string) (App, error) {
	R.l.RLock()
	defer R.l.RUnlock()

	if v, ok := R.Apps[id]; ok {
		return v, nil
	} else {
		return App{}, fmt.Errorf("no App %s", id)
	}
}

func GetPodCgroupNewTaskIdsFromPodCtx(podMeta *protocol.PodContext, tasksMap map[int32]struct{}) []int32 {
	var taskIds []int32

	for containerId, v := range podMeta.Request.ContainerTaskIds {
		containerDir, err := koordletutil.GetContainerCgroupParentDirByID(podMeta.Request.CgroupParent, containerId)
		if err != nil {
			klog.Errorf("container %s lost during reconcile", containerDir)
			continue
		}
		ids, err := GetNewTaskIds(v, tasksMap)
		if err != nil {
			klog.Warningf("failed to get pod container cgroup task ids for container %s/%s/%s, err: %s",
				podMeta.Request.PodMeta.Name, containerId)
			continue
		}
		taskIds = append(taskIds, ids...)
	}
	return taskIds
}

func GetNewTaskIds(ids []int32, tasksMap map[int32]struct{}) ([]int32, error) {
	if tasksMap == nil {
		return ids, nil
	}

	// only append the non-mapped ids
	var taskIDs []int32
	for _, id := range ids {
		if _, ok := tasksMap[id]; !ok {
			taskIDs = append(taskIDs, id)
		}
	}
	return taskIDs, nil
}

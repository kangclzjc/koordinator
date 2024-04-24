package util

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"k8s.io/klog/v2"

	apiext "github.com/koordinator-sh/koordinator/apis/extension"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/runtimehooks/protocol"
	koordletutil "github.com/koordinator-sh/koordinator/pkg/koordlet/util"
	sysutil "github.com/koordinator-sh/koordinator/pkg/koordlet/util/system"
)

type ProtocolUpdater interface {
	Name() string
	Key() string
	Value() string
	Update() error
	SetKey(key string)
	SetValue(key string)
}

const ClosdIdPrefix = "koordlet-"

type App struct {
	Resctrl *sysutil.ResctrlSchemataRaw
	// Hooks   Hook
	Closid     string
	Annotation string
}

type ResctrlEngine interface {
	Rebuild() // rebuild the current control group
	RegisterApp(podid, annotation string, fromNRI bool, updater ProtocolUpdater) error
	UnRegisterApp(podid string, fromNRI bool, updater ProtocolUpdater) error
	GetApp(podid string) (App, bool)
	GetApps() map[string]App
}

func NewRDTEngine(createUpdater Updater, schemataUpdater SchemataUpdater, removeUpdater Updater) (ResctrlEngine, error) {
	var CatL3CbmMask string
	var err error
	if CatL3CbmMask, err = sysutil.ReadCatL3CbmString(); err != nil {
		klog.Errorf("get l3 cache bit mask error: %v", err)
		return nil, err
	}

	if len(CatL3CbmMask) <= 0 {
		return nil, fmt.Errorf("failed to get cat l3 cbm, cbm is empty")
	}
	cbmValue, err := strconv.ParseUint(CatL3CbmMask, 16, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cat l3 cbm %s, err: %v", CatL3CbmMask, err)
	}
	cbm := uint(cbmValue)

	return &RDTEngine{
		Apps:       make(map[string]App),
		CtrlGroups: make(map[string]apiext.Resctrl),
		CBM:        cbm,
		Cgm:        NewControlGroupManager(createUpdater, schemataUpdater, removeUpdater),
	}, nil
}

type RDTEngine struct {
	Apps       map[string]App
	Cgm        ControlGroupManager
	CtrlGroups map[string]apiext.Resctrl
	l          sync.RWMutex
	CBM        uint
}

func (R *RDTEngine) UnRegisterApp(podid string, fromNRI bool, updater ProtocolUpdater) error {
	R.l.Lock()
	defer R.l.Unlock()
	R.Cgm.RemovePod(podid, fromNRI, updater)

	if _, ok := R.Apps[podid]; !ok {
		return fmt.Errorf("pod %s not registered", podid)
	}
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
	R.l.RLock()
	defer R.l.RUnlock()
	R.Cgm.Init()
	for podid, item := range R.Cgm.rdtcgs.Items() {
		v, ok := item.Object.(*ControlGroup)
		if !ok {
			continue
		}

		ids, _ := sysutil.CacheIdsCacheFunc()
		schemataRaw := sysutil.NewResctrlSchemataRaw(ids).WithL3Num(len(ids))
		err := schemataRaw.ParseResctrlSchemata(v.Schemata, -1)
		if err != nil {
			klog.Errorf("failed to parse %v", err)
		}
		R.Apps[podid] = App{
			Resctrl: schemataRaw,
			Closid:  v.Groupid,
		}
	}
	// get resctrl filesystem root
	//root := sysutil.GetResctrlSubsystemDirPath()
	//
	//files, err := os.ReadDir(root)
	//if err != nil {
	//	klog.Errorf("read %s failed err is %v", root, err)
	//	return
	//}
	//
	//for _, file := range files {
	//	if file.IsDir() && strings.HasPrefix(file.Name(), ClosdIdPrefix) {
	//		path := filepath.Join(root, file.Name(), "schemata")
	//		if _, err := os.Stat(path); err == nil {
	//			reader, err := os.Open(path)
	//			if err != nil {
	//				klog.Errorf("open resctrl file path fail, %v", err)
	//			}
	//			content, err := io.ReadAll(reader)
	//			if err != nil {
	//				klog.Errorf("read resctrl file path fail, %v", err)
	//				continue
	//			}
	//			schemata := string(content)
	//			ids, _ := sysutil.CacheIdsCacheFunc()
	//			schemataRaw := sysutil.NewResctrlSchemataRaw(ids).WithL3Num(len(ids))
	//			err = schemataRaw.ParseResctrlSchemata(schemata, -1)
	//			if err != nil {
	//				klog.Errorf("failed to parse %v", err)
	//			}
	//			podid := strings.TrimPrefix(file.Name(), ClosdIdPrefix)
	//			R.l.Lock()
	//			R.Apps[podid] = App{
	//				Resctrl: schemataRaw,
	//				Closid:  file.Name(),
	//			}
	//			R.l.Unlock()
	//		}
	//	}
	//}
}

func (R *RDTEngine) RegisterApp(podid, annotation string, fromNRI bool, updater ProtocolUpdater) error {
	// Parse the JSON value into the BlockIO struct
	var res apiext.ResctrlConfig
	err := json.Unmarshal([]byte(annotation), &res)
	if err != nil {
		klog.Errorf("error is %v", err)
		return err
	}

	schemata := ParseSchemata(res, R.CBM)
	app := App{
		Resctrl:    schemata,
		Closid:     ClosdIdPrefix + podid,
		Annotation: annotation,
	}

	items := []string{}
	for _, item := range []struct {
		validFunc func() (bool, string)
		value     func() string
	}{
		{validFunc: app.Resctrl.ValidateL3, value: app.Resctrl.L3String},
		{validFunc: app.Resctrl.ValidateMB, value: app.Resctrl.MBString},
	} {
		if valid, _ := item.validFunc(); valid {
			items = append(items, item.value())
		}
	}
	schemataStr := strings.Join(items, "")
	if updater != nil {
		updater.SetKey(ClosdIdPrefix + podid)
		updater.SetValue(schemataStr)
		updater.Update()
	}
	R.Cgm.AddPod(podid, schemataStr, fromNRI, updater, nil)

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

func ParseSchemata(config apiext.ResctrlConfig, cbm uint) *sysutil.ResctrlSchemataRaw {
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

	if config.LLC.Schemata.Range != nil && len(config.LLC.Schemata.Range) == 2 {
		start := config.LLC.Schemata.Range[0]
		end := config.LLC.Schemata.Range[1]

		l3MaskValue, err := sysutil.CalculateCatL3MaskValue(cbm, int64(start), int64(end))
		if err != nil {
			klog.Warningf("failed to calculate l3 cat schemata err: %v", err)
			return schemataRaw
		}

		schemataRaw.WithL3Num(len(ids)).WithL3Mask(l3MaskValue)
	}

	if config.LLC.SchemataPerCache != nil {
		for _, v := range config.LLC.SchemataPerCache {
			if len(v.Range) == 2 {
				start := v.Range[0]
				end := v.Range[1]
				l3MaskValue, err := sysutil.CalculateCatL3MaskValue(cbm, int64(start), int64(end))
				if err != nil {
					klog.Warningf("failed to calculate l3 cat schemata err: %v", err)
					return schemataRaw
				}
				// l3 mask MUST be a valid hex
				maskValue, err := strconv.ParseInt(strings.TrimSpace(l3MaskValue), 16, 64)
				if err != nil {
					klog.V(5).Infof("failed to parse l3 mask %s, err: %v", l3MaskValue, err)
				}
				schemataRaw.L3[v.CacheID] = maskValue
			}
		}
	}
	return schemataRaw
}

func (R *RDTEngine) GetApp(id string) (App, bool) {
	R.l.RLock()
	defer R.l.RUnlock()

	if v, ok := R.Apps[id]; ok {
		return v, true
	} else {
		return App{}, false
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

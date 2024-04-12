package util

import (
	sysutil "github.com/koordinator-sh/koordinator/pkg/koordlet/util/system"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

const (
	Remove                   = "Remove"
	Add                      = "Add"
	ResctrlPath              = "/sys/fs/resctrl"
	StatusAliveTimeThreshold = 10
)

type Updater interface {
	Update(string) error
}

type SchemataUpdater interface {
	Update(id, schemata string) error
}

type ControlGroup struct {
	Appid           string
	Groupid         string
	Schemata        string
	Status          string
	StatusAliveTime int
}

type ControlGroupManager struct {
	rdtcgs            map[string]*ControlGroup
	reconcileInterval int64
	CreateUpdater     Updater
	SchemataUpdater   SchemataUpdater
	RemoveUpdater     Updater
	sync.Mutex
}

func NewControlGroupManager(createUpdater Updater, schemataUpdater SchemataUpdater, removeUpdater Updater) ControlGroupManager {
	return ControlGroupManager{
		CreateUpdater:   createUpdater,
		SchemataUpdater: schemataUpdater,
		RemoveUpdater:   removeUpdater,
	}
}

func (c *ControlGroupManager) Init() {
	// initialize based on app information and ctrl group status
	// Load all ctrl groups and
	files, err := os.ReadDir(ResctrlPath)
	if err != nil {
		klog.Errorf("read %s failed err is %v", ResctrlPath, err)
		return
	}
	for _, file := range files {
		// rebuild c.rdtcgs
		if file.IsDir() && strings.HasPrefix(file.Name(), ClosdIdPrefix) {
			path := filepath.Join(ResctrlPath, file.Name(), "schemata")
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
				c.rdtcgs[podid] = &ControlGroup{
					Appid:           podid,
					Groupid:         file.Name(),
					Schemata:        schemata,
					Status:          Add,
					StatusAliveTime: 0,
				}
			}
		}
	}
}

func (c *ControlGroupManager) reconcile() {
	c.Lock()
	defer c.Unlock()

	for app, cg := range c.rdtcgs {
		cg.StatusAliveTime++
		if cg.StatusAliveTime >= StatusAliveTimeThreshold {
			cg.StatusAliveTime = StatusAliveTimeThreshold
		}
		if cg.Status == Remove {
			if cg.Groupid != "" {
				if c.RemoveUpdater != nil {
					err := c.RemoveUpdater.Update(cg.Groupid)
					if err != nil {
						klog.Errorf("remove updater fail %v", err)
					}
				}
			}
			if cg.StatusAliveTime == StatusAliveTimeThreshold {
				delete(c.rdtcgs, app)
			}
		}
	}
}

func (c *ControlGroupManager) Start(stopCh <-chan struct{}) {
	go wait.Until(c.reconcile, time.Duration(c.reconcileInterval), stopCh)
	c.reconcile()
}

func (c *ControlGroupManager) AddPod(podid string, schemata string, fromNRI bool) {
	c.Lock()
	defer c.Unlock()
	pod, ok := c.rdtcgs[podid]
	if !ok {
		pod = &ControlGroup{
			Appid:           podid,
			Groupid:         "",
			Schemata:        schemata,
			Status:          Add,
			StatusAliveTime: 0,
		}
		c.rdtcgs[podid] = pod
	} else {
		if (pod.StatusAliveTime == StatusAliveTimeThreshold) && pod.Status == Remove {
			pod.Status = Add
			pod.StatusAliveTime = 0
		}
	}

	if pod.Status == Add && pod.Groupid == "" {
		if c.CreateUpdater != nil {
			err := c.CreateUpdater.Update(ClosdIdPrefix + podid)
			if err != nil {
				klog.Errorf("create ctrl group error %v", err)
			} else {
				pod.Groupid = ClosdIdPrefix + podid
			}
		}

		if c.SchemataUpdater != nil {
			err := c.SchemataUpdater.Update(podid, schemata)
			if err != nil {
				klog.Errorf("updater ctrl group schemata error %v", err)
			} else {
				pod.Schemata = schemata
			}
		}
		// Create Ctrl Group and Update Schemata
	} else {
		if pod.Status == Add && pod.Groupid != "" {
			if !fromNRI {
				// Update Schemata
				if c.SchemataUpdater != nil {
					err := c.SchemataUpdater.Update(podid, schemata)
					if err != nil {
						klog.Errorf("updater ctrl group schemata error %v", err)
					} else {
						pod.Schemata = schemata
					}
				}
			}
		}
	}
}

func (c *ControlGroupManager) RemovePod(podid string, fromNRI bool) {
	c.Lock()
	defer c.Unlock()

	// RemovePendingPods.Add(pod) => add a special
	pod, ok := c.rdtcgs[podid]
	if !ok {
		pod = &ControlGroup{podid, "", "", Remove, 0}
		c.rdtcgs[podid] = pod
	}

	if !ok {
		return
	}
	if (fromNRI || pod.StatusAliveTime == StatusAliveTimeThreshold) && pod.Status == Add {
		pod.Status = Remove
		pod.StatusAliveTime = 0
	}
}

package util

import (
	sysutil "github.com/koordinator-sh/koordinator/pkg/koordlet/util/system"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	Remove      = "Remove"
	Add         = "Add"
	ResctrlPath = "/sys/fs/resctrl"
	Ttl         = 10
	PREFIX      = "koordlet_"
)

type Updater interface {
	Update(string) error
}

type SchemataUpdater interface {
	Update(id, schemata string) error
}

type ControlGroup struct {
	Appid    string
	Groupid  string
	Schemata string
	Status   string
	Ttl      int
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

func (c *ControlGroupManager) reconcile() {
	c.Lock()
	defer c.Unlock()

	for app, cg := range c.rdtcgs {
		cg.Ttl--
		if cg.Ttl <= 0 {
			cg.Ttl = 0
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
			if cg.Ttl == 0 {
				delete(c.rdtcgs, app)
			}
		}
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
		if file.IsDir() && strings.HasPrefix(file.Name(), PREFIX) {
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
				podid := strings.TrimPrefix(file.Name(), PREFIX)
				c.rdtcgs[podid] = &ControlGroup{
					Appid:    podid,
					Groupid:  file.Name(),
					Schemata: schemata,
					Status:   Add,
					Ttl:      10,
				}
			}
		}
	}
}

func (c *ControlGroupManager) Start(stopCh <-chan struct{}) {
	go wait.Until(c.reconcile, time.Duration(c.reconcileInterval), stopCh)
	c.reconcile()
}

// Return @1 means need to create group, @2 means need to update schemata
func (c *ControlGroupManager) AddPod(podid string, schemata string, trust bool, updater Updater) {
	c.Lock()
	defer c.Unlock()
	pod, ok := c.rdtcgs[podid]
	if !ok {
		pod = &ControlGroup{
			Appid:    podid,
			Groupid:  "",
			Schemata: schemata,
			Status:   Add,
			Ttl:      Ttl,
		}
		c.rdtcgs[podid] = pod
	} else {
		if (trust || pod.Ttl == 0) && pod.Status == Remove {
			pod.Status = Add
			pod.Ttl = Ttl
		}
	}

	if pod.Status == Add && pod.Groupid == "" {
		if c.CreateUpdater != nil {
			err := c.CreateUpdater.Update(PREFIX + podid)
			if err != nil {
				klog.Errorf("create ctrl group error %v", err)
			} else {
				pod.Groupid = PREFIX + podid
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
			if !trust {
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

func (c *ControlGroupManager) RemovePod(podid string, trust bool) {
	c.Lock()
	defer c.Unlock()

	// RemovePendingPods.Add(pod) => add a special
	pod, ok := c.rdtcgs[podid]
	if !ok {
		pod = &ControlGroup{podid, "", "", Remove, Ttl}
		c.rdtcgs[podid] = pod
	}

	if (trust || pod.Ttl == 0) && pod.Status == Add {
		pod.Status = Remove
		pod.Ttl = 10
	}
}

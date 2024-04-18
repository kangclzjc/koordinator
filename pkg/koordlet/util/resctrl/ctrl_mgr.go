package util

import (
	"github.com/koordinator-sh/koordinator/pkg/koordlet/metricsadvisor/framework"
	gocache "github.com/patrickmn/go-cache"
	"io/ioutil"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	Appid    string
	Groupid  string
	Schemata string
	Status   string
}

type ControlGroupManager struct {
	rdtcgs *gocache.Cache
	//rdtcgs            map[string]*ControlGroup
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
		rdtcgs:          gocache.New(StatusAliveTimeThreshold, framework.CleanupInterval),
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
				//ids, _ := sysutil.CacheIdsCacheFunc()
				//schemataRaw := sysutil.NewResctrlSchemataRaw(ids).WithL3Num(len(ids))
				//err = schemataRaw.ParseResctrlSchemata(schemata, -1)
				//if err != nil {
				//	klog.Errorf("failed to parse %v", err)
				//}
				podid := strings.TrimPrefix(file.Name(), ClosdIdPrefix)
				c.rdtcgs.Set(podid, &ControlGroup{
					Appid:    podid,
					Groupid:  file.Name(),
					Schemata: schemata,
					Status:   Add,
				}, gocache.DefaultExpiration)
			}
		}
	}
}

func (c *ControlGroupManager) AddPod(podid string, schemata string, fromNRI bool) {
	c.Lock()
	defer c.Unlock()
	p, ok := c.rdtcgs.Get(podid)
	pod := p.(*ControlGroup)
	if !ok {
		pod = &ControlGroup{
			Appid:    podid,
			Groupid:  "",
			Schemata: "",
			Status:   Add,
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

		c.rdtcgs.Set(podid, pod, gocache.DefaultExpiration)
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
				c.rdtcgs.Set(podid, pod, gocache.DefaultExpiration)
			}
		}
	}
}

func (c *ControlGroupManager) RemovePod(podid string, fromNRI bool) {
	c.Lock()
	defer c.Unlock()

	// RemovePendingPods.Add(pod) => add a special
	p, ok := c.rdtcgs.Get(podid)
	pod := p.(*ControlGroup)
	if !ok {
		pod = &ControlGroup{podid, "", "", Remove}
		err := c.RemoveUpdater.Update(podid)
		if err != nil {
			klog.Errorf("remove updater fail %v", err)
		}
		c.rdtcgs.Set(podid, pod, gocache.DefaultExpiration)
		return
	}

	if fromNRI && pod.Status == Add {
		pod.Status = Remove
		err := c.RemoveUpdater.Update(podid)
		if err != nil {
			klog.Errorf("remove updater fail %v", err)
		}
		c.rdtcgs.Set(podid, pod, gocache.DefaultExpiration)
	}
}

package util

import (
	"github.com/koordinator-sh/koordinator/pkg/koordlet/metricsadvisor/framework"
	sysutil "github.com/koordinator-sh/koordinator/pkg/koordlet/util/system"
	gocache "github.com/patrickmn/go-cache"
	"io"
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
	rdtcgs            *gocache.Cache
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
	c.Lock()
	defer c.Unlock()
	// get resctrl filesystem root
	root := sysutil.GetResctrlSubsystemDirPath()
	files, err := os.ReadDir(root)
	if err != nil {
		klog.Errorf("read %s failed err is %v", root, err)
		return
	}
	for _, file := range files {
		// rebuild c.rdtcgs
		if file.IsDir() && strings.HasPrefix(file.Name(), ClosdIdPrefix) {
			path := filepath.Join(root, file.Name(), "schemata")
			if _, err := os.Stat(path); err == nil {
				reader, err := os.Open(path)
				if err != nil {
					klog.Errorf("open resctrl file path fail, %v", err)
				}
				content, err := io.ReadAll(reader)
				if err != nil {
					klog.Errorf("read resctrl file path fail, %v", err)
					return
				}
				schemata := string(content)
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

func (c *ControlGroupManager) AddPod(podid string, schemata string, fromNRI bool, createUpdater ProtocolUpdater, schemataUpdater ProtocolUpdater) {
	c.Lock()
	defer c.Unlock()
	p, ok := c.rdtcgs.Get(podid)

	var pod *ControlGroup
	if !ok {
		pod = &ControlGroup{
			Appid:    podid,
			Groupid:  "",
			Schemata: "",
			Status:   Add,
		}
	} else {
		pod = p.(*ControlGroup)
	}

	if pod.Status == Add && pod.Groupid == "" {
		if createUpdater != nil {
			err := createUpdater.Update()
			if err != nil {
				klog.Errorf("create ctrl group error %v", err)
			} else {
				pod.Groupid = ClosdIdPrefix + podid
			}
		}

		if schemataUpdater != nil {
			err := schemataUpdater.Update()
			if err != nil {
				klog.Errorf("updater ctrl group schemata error %v", err)
			}
			pod.Schemata = schemata
		}

		c.rdtcgs.Set(podid, pod, gocache.DefaultExpiration)
		// Create Ctrl Group and Update Schemata
	} else {
		if pod.Status == Add && pod.Groupid != "" {
			if !fromNRI {
				// Update Schemata
				if schemataUpdater != nil {
					err := schemataUpdater.Update()
					if err != nil {
						klog.Errorf("updater ctrl group schemata error %v", err)
					}
					pod.Schemata = schemata
				}
				c.rdtcgs.Set(podid, pod, gocache.DefaultExpiration)
			}
		}
	}
}

func (c *ControlGroupManager) RemovePod(podid string, fromNRI bool, removeUpdater ProtocolUpdater) {
	c.Lock()
	defer c.Unlock()

	// RemovePendingPods.Add(pod) => add a special
	p, ok := c.rdtcgs.Get(podid)
	if !ok {
		pod := &ControlGroup{podid, "", "", Remove}
		if removeUpdater != nil {
			err := removeUpdater.Update()
			if err != nil {
				klog.Errorf("remove updater fail %v", err)
			}
		}

		c.rdtcgs.Set(podid, pod, gocache.DefaultExpiration)
		return
	}
	pod := p.(*ControlGroup)
	if fromNRI && pod.Status == Add {
		pod.Status = Remove
		err := removeUpdater.Update()
		if err != nil {
			klog.Errorf("remove updater fail %v", err)
		}
		c.rdtcgs.Set(podid, pod, gocache.DefaultExpiration)
	}
}

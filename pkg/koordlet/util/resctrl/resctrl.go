package util

import (
	"encoding/json"
	"fmt"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/resourceexecutor"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/statesinformer"
	koordletutil "github.com/koordinator-sh/koordinator/pkg/koordlet/util"
	sysutil "github.com/koordinator-sh/koordinator/pkg/koordlet/util/system"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"strings"
)

var cgroupReader = resourceexecutor.CgroupV2Reader{}

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
	Rebuild() map[string]App // rebuild the current control group
	GetCurrentCtrlGroups() map[string]Resctrl
	Config(schemata string) // TODO:@Bowen use schemata or use policy to parse this string?
	GetConfig() map[string]string
	RegisterApp(podid, annotation string) error
	GetApp(podid string) (App, error)
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
	Policy     ResctrlPolicy
}

func (R *RDTEngine) Rebuild() map[string]App {
	// 获取 resctrl 文件系统根目录
	root := sysutil.GetResctrlSubsystemDirPath()

	// 遍历根目录下的所有目录
	files, err := os.ReadDir(root)
	if err != nil {
		klog.Errorf("read %s failed err is %v", root, err)
		return nil
	}

	for _, file := range files {
		// 判断是否是目录
		if file.IsDir() && strings.HasPrefix(file.Name(), "koordlet") {
			// 判断是否是控制组
			path := filepath.Join(root, file.Name(), "schemata")
			if _, err := os.Stat(path); err == nil {
				content, err := ioutil.ReadFile(path)
				if err != nil {
					fmt.Println(err)
					return R.Apps
				}
				schemata := string(content)
				ids, _ := sysutil.CacheIdsCacheFunc()
				schemataRaw := sysutil.NewResctrlSchemataRaw(ids).WithL3Num(len(ids))
				err = schemataRaw.ParseResctrlSchemata(schemata, -1)
				if err != nil {
					klog.Errorf("failed to parse %v", err)
				}
				podid := strings.TrimPrefix(file.Name(), "koordlet-")
				R.Apps[podid] = App{
					Resctrl: schemataRaw,
					Closid:  file.Name(),
				}
			}
		}
	}
	return R.Apps
}

func (R *RDTEngine) GetCurrentCtrlGroups() map[string]Resctrl {
	//TODO implement me
	panic("implement me")
}

func (R *RDTEngine) Config(config string) {
	//TODO implement me
}

func (R *RDTEngine) GetConfig() map[string]string {
	//TODO implement me
	panic("implement me")
}

// annotation is resctl string
func (R *RDTEngine) RegisterApp(podid, annotation string) error {
	if _, ok := R.Apps[podid]; ok {
		return fmt.Errorf("pod %s already registered", podid)
	}
	// Parse the JSON value into the BlockIO struct
	var res ResctrlConfig
	err := json.Unmarshal([]byte(annotation), &res)
	if err != nil {
		klog.Errorf("error is %v", err)
		//panic(err)
		return nil
	}

	// Print the parsed data
	klog.Infof("resctrl: %v", res)
	if res.MB.Schemata.Percent != 0 && res.MB.Schemata.Range != nil {
		klog.Infof("resctrl MB is : %v", res.MB)
	}

	schemata := ParseSchemata(res)
	app := App{
		Resctrl: schemata,
		Closid:  "koordlet-" + podid,
	}
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
	if v, ok := R.Apps[id]; ok {
		return v, nil
	} else {
		return App{}, fmt.Errorf("no App %s", id)
	}
}

// TODO:@Bowen use policy to change some action in the future? Any ideas?
type ResctrlPolicy interface {
}

func GetPodCgroupNewTaskIds(podMeta *statesinformer.PodMeta, tasksMap map[int32]struct{}) []int32 {
	var taskIds []int32

	pod := podMeta.Pod
	containerMap := make(map[string]*corev1.Container, len(pod.Spec.Containers))
	for i := range pod.Spec.Containers {
		container := &pod.Spec.Containers[i]
		containerMap[container.Name] = container
	}
	for _, containerStat := range pod.Status.ContainerStatuses {
		// reconcile containers
		container, exist := containerMap[containerStat.Name]
		if !exist {
			klog.Warningf("container %s/%s/%s lost during reconcile resctrl group", pod.Namespace,
				pod.Name, containerStat.Name)
			continue
		}

		containerDir, err := koordletutil.GetContainerCgroupParentDir(podMeta.CgroupDir, &containerStat)
		if err != nil {
			klog.V(4).Infof("failed to get pod container cgroup path for container %s/%s/%s, err: %s",
				pod.Namespace, pod.Name, container.Name, err)
			continue
		}

		klog.Infof("--------------- current ContainerDir is %s", containerDir)
		ids, err := GetContainerCgroupNewTaskIds(containerDir, tasksMap)
		if err != nil {
			klog.Warningf("failed to get pod container cgroup task ids for container %s/%s/%s, err: %s",
				pod.Namespace, pod.Name, container.Name, err)
			continue
		}
		taskIds = append(taskIds, ids...)
	}

	// try retrieve task IDs from the sandbox container, especially for VM-based container runtime
	sandboxID, err := koordletutil.GetPodSandboxContainerID(pod)
	if err != nil {
		klog.V(4).Infof("failed to get sandbox container ID for pod %s/%s, err: %s",
			pod.Namespace, pod.Name, err)
		return taskIds
	}
	sandboxContainerDir, err := koordletutil.GetContainerCgroupParentDirByID(podMeta.CgroupDir, sandboxID)
	if err != nil {
		klog.V(4).Infof("failed to get pod container cgroup path for sandbox container %s/%s/%s, err: %s",
			pod.Namespace, pod.Name, sandboxID, err)
		return taskIds
	}
	ids, err := GetContainerCgroupNewTaskIds(sandboxContainerDir, tasksMap)
	if err != nil {
		klog.Warningf("failed to get pod container cgroup task ids for sandbox container %s/%s/%s, err: %s",
			pod.Namespace, pod.Name, sandboxID, err)
		return taskIds
	}
	taskIds = append(taskIds, ids...)

	return taskIds
}

func GetContainerCgroupNewTaskIds(containerParentDir string, tasksMap map[int32]struct{}) ([]int32, error) {
	ids, err := cgroupReader.ReadCPUTasks(containerParentDir)
	if err != nil && resourceexecutor.IsCgroupDirErr(err) {
		klog.V(5).Infof("failed to read container task ids whose cgroup path %s does not exists, err: %s",
			containerParentDir, err)
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to read container task ids, err: %w", err)
	}

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

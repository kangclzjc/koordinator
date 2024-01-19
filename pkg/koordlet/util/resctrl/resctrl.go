package util

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/koordinator-sh/koordinator/pkg/koordlet/resourceexecutor"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/statesinformer"
	koordletutil "github.com/koordinator-sh/koordinator/pkg/koordlet/util"
)

var cgroupReader = resourceexecutor.NewCgroupReader()

type Resctrl struct {
	L3 map[int]string
	MB map[int]string
}

type App struct {
	Resctrl Resctrl
	// Hooks   Hook
	Closid string
}

// TODO: @Bowen we should talk about this interface functions' meaning?
type ResctrlEngine interface {
	Rebuild() // rebuild the current control group
	GetCurrentCtrlGroups() map[string]Resctrl
	Config(schemata string) // TODO:@Bowen use schemata or use policy to parse this string?
	GetConfig() map[string]string
	RegisterApp(podid, annotation, closid string) error
	GetApp(podid string) (App, error)
}

func NewRDTEngine() RDTEngine {
	return RDTEngine{
		Apps:       make(map[string]App),
		CtrlGroups: make(map[string]Resctrl),
	}
}

type RDTEngine struct {
	Apps       map[string]App
	CtrlGroups map[string]Resctrl
	Policy     ResctrlPolicy
}

func (R RDTEngine) Rebuild() {
	//TODO implement me
}

func (R RDTEngine) GetCurrentCtrlGroups() map[string]Resctrl {
	//TODO implement me
	panic("implement me")
}

func (R RDTEngine) Config(config string) {
	//TODO implement me
	panic("implement me")
}

func (R RDTEngine) GetConfig() map[string]string {
	//TODO implement me
	panic("implement me")
}

// annotation is resctl string
func (R RDTEngine) RegisterApp(podid, annotation, closid string) error {
	app := App{
		Resctrl: Resctrl{},
		Closid:  closid,
	}
	R.Apps[podid] = app
	return nil
}

func (R RDTEngine) GetApp(id string) (App, error) {
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

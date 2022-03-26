package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"

	"github.com/gocrane/crane/pkg/known"
)

const (
	ExtResourcePrefixFormat = "gocrane.io/%s"
)

// IsPodAvailable returns true if a pod is available; false otherwise.
// copied from k8s.io/kubernetes/pkg/api/v1/pod.go
func IsPodAvailable(pod *v1.Pod, minReadySeconds int32, now metav1.Time) bool {
	if !IsPodReady(pod) {
		return false
	}

	c := GetPodReadyCondition(pod.Status)
	minReadySecondsDuration := time.Duration(minReadySeconds) * time.Second
	if minReadySeconds == 0 || (!c.LastTransitionTime.IsZero() && c.LastTransitionTime.Add(minReadySecondsDuration).Before(now.Time)) {
		return true
	}
	return false
}

// IsPodReady returns true if a pod is ready; false otherwise.
// copied from k8s.io/kubernetes/pkg/api/v1/pod.go
func IsPodReady(pod *v1.Pod) bool {
	condition := GetPodReadyCondition(pod.Status)
	return condition != nil && condition.Status == v1.ConditionTrue
}

// GetPodReadyCondition extracts the pod ready condition from the given status and returns that.
// Returns nil if the condition is not present.
// copied from k8s.io/kubernetes/pkg/api/v1/pod.go
func GetPodReadyCondition(status v1.PodStatus) *v1.PodCondition {
	_, condition := GetPodCondition(&status, v1.PodReady)
	return condition
}

// GetPodCondition extracts the provided condition from the given status and returns that.
// Returns nil and -1 if the condition is not present, and the index of the located condition.
// copied from k8s.io/kubernetes/pkg/api/v1/pod.go
func GetPodCondition(status *v1.PodStatus, conditionType v1.PodConditionType) (int, *v1.PodCondition) {
	if status == nil {
		return -1, nil
	}
	if status.Conditions == nil {
		return -1, nil
	}
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return i, &status.Conditions[i]
		}
	}
	return -1, nil
}

// EvictPodWithGracePeriod evict pod with grace period
func EvictPodWithGracePeriod(client clientset.Interface, pod *v1.Pod, gracePeriodSeconds *int32) error {
	if kubelettypes.IsCriticalPod(pod) {
		return fmt.Errorf("eviction manager: cannot evict a critical pod(%s)", klog.KObj(pod))
	}

	var grace = GetInt64withDefault(pod.Spec.TerminationGracePeriodSeconds, known.DefaultDeletionGracePeriodSeconds)
	if gracePeriodSeconds != nil {
		grace = int64(*gracePeriodSeconds)
	}

	e := &policyv1beta1.Eviction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		DeleteOptions: metav1.NewDeleteOptions(grace),
	}

	return client.CoreV1().Pods(pod.Namespace).EvictV1beta1(context.Background(), e)
}

func EvictPodForExtResource(client clientset.Interface, pod *v1.Pod) error {
	ref := metav1.GetControllerOfNoCopy(pod)
	deleteLabel := "gocrane.io/specified-delete"
	if ref != nil {
		if ref.Kind == "CloneSet" {
			deleteLabel = "apps.kruise.io/specified-delete"
		}
	}

	deletePath := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				deleteLabel: "true",
			},
		},
	}
	jsonPatch, err := json.Marshal(deletePath)
	if err != nil {
		klog.Errorf("Failed to generate jsonPatch, %v", err)
		return err
	}
	klog.V(4).Infof("jsonPatch: %s", jsonPatch)

	// patch pod delete-label info
	if _, err := client.CoreV1().Pods(pod.Namespace).Patch(context.TODO(), pod.Name, types.MergePatchType, jsonPatch, metav1.PatchOptions{}, "status"); err != nil {
		klog.Errorf("Failed to patch pod %s's delete-label, %v", pod.Name, err)
		return err
	}
	return nil
}

// CalculatePodRequests sum request total from pods
func CalculatePodRequests(pods []v1.Pod, resource v1.ResourceName) (int64, error) {
	var requests int64
	for _, pod := range pods {
		for _, c := range pod.Spec.Containers {
			if containerRequest, ok := c.Resources.Requests[resource]; ok {
				requests += containerRequest.MilliValue()
			} else {
				return 0, fmt.Errorf("missing request for %s", resource)
			}
		}
	}
	return requests, nil
}

// GetPodContainerByName get container info by container name
func GetPodContainerByName(pod *v1.Pod, containerName string) (v1.Container, error) {
	for _, v := range pod.Spec.Containers {
		if v.Name == containerName {
			return v, nil
		}
	}

	return v1.Container{}, fmt.Errorf("container not found")
}

// CalculatePodTemplateRequests sum request total from podTemplate
func CalculatePodTemplateRequests(podTemplate *v1.PodTemplateSpec, resource v1.ResourceName) (int64, error) {
	var requests int64
	for _, c := range podTemplate.Spec.Containers {
		if containerRequest, ok := c.Resources.Requests[resource]; ok {
			requests += containerRequest.MilliValue()
		} else {
			return 0, fmt.Errorf("missing request for %s", resource)
		}
	}

	return requests, nil
}

// GetExtCpuRes get container's gocrane.io/cpu usage
func GetExtCpuRes(container v1.Container) (resource.Quantity, bool) {
	for res, val := range container.Resources.Limits {
		if strings.HasPrefix(res.String(), fmt.Sprintf(ExtResourcePrefixFormat, v1.ResourceCPU)) {
			return val, true
		}
	}
	return resource.Quantity{}, false
}

func GetContainerNameFromPod(pod *v1.Pod, containerId string) string {
	if containerId == "" {
		return ""
	}

	for _, v := range pod.Status.ContainerStatuses {
		strList := strings.Split(v.ContainerID, "//")
		if len(strList) > 0 {
			if strList[len(strList)-1] == containerId {
				return v.Name
			}
		}
	}

	return ""
}

func GetContainerFromPod(pod *v1.Pod, containerName string) *v1.Container {
	if containerName == ""{
		return nil
	}
	for _, v := range pod.Spec.Containers {
		if v.Name == containerName {
			return &v
		}
	}
	return nil
}

// GetExtCpuRes get container's gocrane.io/cpu usage
func GetContainerExtCpuResFromPod(pod *v1.Pod, containerName string) (resource.Quantity, bool) {
	c := GetContainerFromPod(pod, containerName)
	if c == nil {
		return resource.Quantity{}, false
	}
	return GetExtCpuRes(*c)
}

func GetPodRequestExtCpuMilliValue(pod *v1.Pod) (int64, bool) {
	var extCpu int64 = 0
	useExtCpu := false
	for _, container := range pod.Spec.Containers {
		if quantity, ok := container.Resources.Requests[v1.ResourceName(fmt.Sprintf(ExtResourcePrefixFormat, v1.ResourceCPU))]; ok {
			useExtCpu = true
			extCpu += quantity.MilliValue()
		}
	}
	return extCpu, useExtCpu
}

func GetContainerStatus(pod *v1.Pod, container v1.Container) v1.ContainerState {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == container.Name {
			return cs.State
		}
	}
	return v1.ContainerState{}
}

func GetContainerIdFromPod(pod *v1.Pod, containerName string) string {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == containerName {
			return GetContainerIdFromKey(cs.ContainerID)
		}
	}
	return ""
}

func IsPodDeleting(pod *v1.Pod) bool {
	if pod.DeletionTimestamp != nil {
		return true
	}
	ref := metav1.GetControllerOfNoCopy(pod)
	deleteLabel := "gocrane.io/specified-delete"
	if ref != nil {
		if ref.Kind == "CloneSet" {
			deleteLabel = "apps.kruise.io/specified-delete"
		}
	}
	if _, ok := pod.Labels[deleteLabel]; ok {
		return true
	}
	return false
}

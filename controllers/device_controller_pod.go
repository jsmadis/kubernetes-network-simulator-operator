/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"github.com/go-logr/logr"
	networksimulatorv1 "github.com/jsmadis/kubernetes-network-simulator-operator/api/v1"
	"github.com/jsmadis/kubernetes-network-simulator-operator/pkg/util"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

// ManageDevicePodLogic manages all logic for device controller about pod, creating, updating deleting...
func (r DeviceReconciler) ManageDevicePodLogic(device networksimulatorv1.Device, ctx context.Context, log logr.Logger) (ctrl.Result, error, bool) {
	if r.IsNamespaceBeingDeleted(device.Spec.NetworkName, ctx) {
		log.V(1).Info("Namespace is being deleted")
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil, false
	}

	if r.isPodBeingDeleted(device, ctx) {
		log.V(1).Info("Pod is being deleted")
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil, false
	}

	if ok := r.deleteOldPod(&device, ctx, log); !ok {
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil, false
	}

	if !r.isPodCreated(device, ctx) {
		_, err := r.createPod(&device, ctx, log)
		if err != nil {
			return ctrl.Result{}, err, false
		}
		return ctrl.Result{}, nil, false
	}

	if r.isPodOutDated(device, ctx) {
		log.V(1).Info("Deleting outdated pod")
		if err := r.deletePod(device, ctx, log); err != nil {
			return ctrl.Result{RequeueAfter: 2 * time.Second}, err, false
		}
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil, false
	}
	return ctrl.Result{}, nil, true
}

// ManageCleanUpPodLogic cleans resources related to pod
func (r DeviceReconciler) ManageCleanUpPodLogic(device networksimulatorv1.Device, ctx context.Context, log logr.Logger) error {
	if r.isPodCreated(device, ctx) {
		if err := r.deletePod(device, ctx, log); err != nil {
			return err
		}
	}
	return nil
}

// checks if the pod exist for the given device
func (r DeviceReconciler) isPodCreated(device networksimulatorv1.Device, ctx context.Context) bool {
	pod, err := r.getPod(device.PodName(), device.Spec.NetworkName, ctx)
	if err != nil {
		return false
	}
	return pod.Name == device.PodName()
}

// isPodBeingDeleted checks if the pod is being deleted
func (r DeviceReconciler) isPodBeingDeleted(device networksimulatorv1.Device, ctx context.Context) bool {
	pod, err := r.getPod(device.PodName(), device.Spec.NetworkName, ctx)
	if err != nil {
		return false
	}
	return util.IsBeingDeleted(pod)
}

// isPodOutDated checks if the pod corresponds to the device
func (r DeviceReconciler) isPodOutDated(device networksimulatorv1.Device, ctx context.Context) bool {
	pod, err := r.getPod(device.PodName(), device.Spec.NetworkName, ctx)
	if err != nil {
		return false
	}
	return !equality.Semantic.DeepDerivative(device.Spec.PodSpec, pod.Spec)
}

// getPod returns pod for given name and namespace
func (r DeviceReconciler) getPod(name string, namespace string, ctx context.Context) (*v1.Pod, error) {
	var pod v1.Pod
	err := r.GetClient().Get(
		ctx,
		types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		},
		&pod)
	if err != nil {
		return nil, err
	}
	return &pod, nil
}

// clearDeviceStatus set status for NetworkName and PodName to empty string
func (r DeviceReconciler) clearDeviceStatus(device *networksimulatorv1.Device, ctx context.Context, log logr.Logger) bool {
	device.Status.NetworkName = ""
	device.Status.PodName = ""
	if err2 := r.updateDeviceStatus(device, ctx, log); err2 != nil {
		log.Error(err2, "unable to clean status from old network and name that doesn't have pod")
		return false
	}
	return true
}

// deleteOldPod when device is moved to another network we need to delete pod from old network
// This function deletes pod from previous network
func (r DeviceReconciler) deleteOldPod(device *networksimulatorv1.Device, ctx context.Context, log logr.Logger) bool {
	if device.Spec.NetworkName == device.Status.NetworkName && device.PodName() == device.Status.PodName {
		return true
	}
	if device.Status.PodName == "" || device.Status.NetworkName == "" {
		return true
	}

	pod, err := r.getPod(device.Status.PodName, device.Status.NetworkName, ctx)
	if err != nil {
		log.V(1).Info("Expected pod not found", "err", err)
		// Pod doesn't exist so we need to remove old statuses
		r.clearDeviceStatus(device, ctx, log)
		return false
	}

	if err := r.GetClient().Delete(ctx, pod); err != nil {
		log.Error(err, "unable to delete old pod", "pod", pod)
		return false
	}

	if !r.clearDeviceStatus(device, ctx, log) {
		return false
	}

	log.Info("Old pod successfully deleted", "pod", pod)
	return false
}

// deletePod deletes pod that was created for the device
func (r DeviceReconciler) deletePod(device networksimulatorv1.Device, ctx context.Context, log logr.Logger) error {
	pod, err := r.getPod(device.PodName(), device.Spec.NetworkName, ctx)
	if err != nil {
		log.Error(err, "unable to find pod to delete")
		return err
	}
	if err := r.GetClient().Delete(ctx, pod); err != nil {
		log.Error(err, "unable to delete pod", "pod", pod)
		return err
	}
	log.V(1).Info("Deleted pod", "pod", pod)
	return nil
}

// createPod creates pod for device
func (r DeviceReconciler) createPod(
	device *networksimulatorv1.Device, ctx context.Context, log logr.Logger) (*v1.Pod, error) {
	name := device.PodName()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        name,
			Namespace:   device.Spec.NetworkName,
		},
		Spec: *device.Spec.PodSpec.DeepCopy(),
	}
	pod.ObjectMeta.Labels["Patriot-Device"] = device.Name
	pod.ObjectMeta.Labels["Patriot"] = "device"
	if err := ctrl.SetControllerReference(device, pod, r.Scheme); err != nil {
		log.V(1).Info("Failed to set controller reference for pod", "pod", pod)
		return nil, err
	}

	if err := r.GetClient().Create(ctx, pod); err != nil {
		log.Error(err, "unable to create Pod for device", "pod", pod)
		return nil, err
	}
	log.V(1).Info("Created pod")

	// update device status
	device.Status.NetworkName = device.Spec.NetworkName
	device.Status.PodName = name
	if err := r.updateDeviceStatus(device, ctx, log); err != nil {
		log.Error(err, "unable to update status for device", "device", device)
	}

	return pod, nil
}

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
	"errors"
	"github.com/go-logr/logr"
	"github.com/jsmadis/kubernetes-network-simulator-operator/pkg/util"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"

	networksimulatorv1 "github.com/jsmadis/kubernetes-network-simulator-operator/api/v1"
)

// DeviceReconciler reconciles a Device object
type DeviceReconciler struct {
	util.ReconcilerBase
}

const DeviceControllerName = "DeviceCRD"

//+kubebuilder:rbac:groups=network-simulator.patriot-framework.io,resources=devices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=network-simulator.patriot-framework.io,resources=devices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=network-simulator.patriot-framework.io,resources=devices/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Device object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *DeviceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("device", req.NamespacedName)

	var device networksimulatorv1.Device
	if err := r.GetClient().Get(ctx, req.NamespacedName, &device); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if ok, err := r.IsValidDevice(&device, ctx); !ok {
		log.Error(err, "Invalid CR of network", "device", device)
		return ctrl.Result{}, err
	}

	if ok := r.IsInitialized(&device); !ok {
		err := r.GetClient().Update(ctx, &device)
		if err != nil {
			log.Error(err, "unable to update instance", "device", device)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if util.IsBeingDeleted(&device) {
		if !util.HasFinalizer(&device, DeviceControllerName) {
			return ctrl.Result{}, nil
		}
		err := r.ManageCleanUpLogic(device, ctx, log)
		if err != nil {
			log.Error(err, "unable to delete device", "device", device)
			return ctrl.Result{}, err
		}
		util.RemoveFinalizer(&device, DeviceControllerName)
		err = r.GetClient().Update(ctx, &device)
		if err != nil {
			log.Error(err, "unable to update device", "device", device)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	return r.ManageOperatorLogic(device, ctx, log)
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeviceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return r.addWatchers(mgr)
}

// addWatchers adds watcher for resources created by device controller
func (r DeviceReconciler) addWatchers(mgr ctrl.Manager) error {
	c, err := controller.New("device-controller", mgr, controller.Options{Reconciler: &r})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &networksimulatorv1.Device{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Predicate func to ignore create and generic events
	p := predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return true
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return true
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	}

	// Watch for pod owned by device
	err = c.Watch(&source.Kind{Type: &v1.Pod{}}, &handler.EnqueueRequestForOwner{
		OwnerType:    &networksimulatorv1.Device{},
		IsController: false,
	},
		p)
	if err != nil {
		return err
	}
	return nil
}

func (r *DeviceReconciler) IsValidDevice(obj metav1.Object, ctx context.Context) (bool, error) {
	deviceCrd, ok := obj.(*networksimulatorv1.Device)
	if !ok {
		return false, nil
	}
	_, err := r.GetNamespace(deviceCrd.Spec.NetworkName, ctx)
	if err != nil {
		return false, errors.New("unable to find namespace of the network")
	}
	return true, nil
}

func (r *DeviceReconciler) IsInitialized(obj metav1.Object) bool {
	networkCrd, ok := obj.(*networksimulatorv1.Device)
	if !ok {
		return false
	}
	if util.HasFinalizer(networkCrd, DeviceControllerName) {
		return true
	}
	util.AddFinalizer(networkCrd, DeviceControllerName)
	return false
}

func (r DeviceReconciler) isPodBeingDeleted(device networksimulatorv1.Device, ctx context.Context) bool {
	pod, err := r.getPod(device, ctx)
	if err != nil {
		return false
	}
	return util.IsBeingDeleted(pod)
}

func (r DeviceReconciler) isPodOutDated(device networksimulatorv1.Device, ctx context.Context) bool {
	pod, err := r.getPod(device, ctx)
	if err != nil {
		return false
	}
	return !equality.Semantic.DeepDerivative(device.Spec.PodTemplate.Spec, pod.Spec)
}

func (r DeviceReconciler) ManageOperatorLogic(
	device networksimulatorv1.Device, ctx context.Context, log logr.Logger) (ctrl.Result, error) {
	if r.IsNamespaceBeingDeleted(device.Spec.NetworkName, ctx) {
		log.V(1).Info("Namespace is being deleted")
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
	}

	if r.isPodBeingDeleted(device, ctx) {
		log.V(1).Info("Pod is being deleted")
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
	}

	if !r.IsPodCreated(device, ctx) {
		_, err := r.createPod(&device, ctx, log)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if r.isPodOutDated(device, ctx) {
		log.V(1).Info("Deleting outdated pod")
		if err := r.deletePod(device, ctx, log); err != nil {
			return ctrl.Result{RequeueAfter: 2 * time.Second}, err
		}
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r DeviceReconciler) ManageCleanUpLogic(device networksimulatorv1.Device,
	ctx context.Context, log logr.Logger) error {
	pod, err := r.getPod(device, ctx)
	if err != nil {
		log.V(1).Info("Unable to get pod when cleaning up", "err", err)
		return err
	}
	if err := r.GetClient().Delete(ctx, pod); err != nil {
		log.Error(err, "unable to delete pod for device when cleaning up")
		return err
	}
	return nil
}

func (r DeviceReconciler) IsPodCreated(device networksimulatorv1.Device, ctx context.Context) bool {
	pod, err := r.getPod(device, ctx)
	if err != nil {
		return false
	}
	return pod.Name == device.Name+"-pod"
}

func (r DeviceReconciler) getPod(device networksimulatorv1.Device, ctx context.Context) (*v1.Pod, error) {
	var pod v1.Pod
	err := r.GetClient().Get(
		ctx,
		types.NamespacedName{
			Namespace: device.Spec.NetworkName,
			Name:      device.Name + "-pod",
		},
		&pod)
	if err != nil {
		return nil, err
	}
	return &pod, nil
}

func (r DeviceReconciler) deletePod(device networksimulatorv1.Device, ctx context.Context, log logr.Logger) error {
	pod, err := r.getPod(device, ctx)
	if err != nil {
		log.Error(err, "unable to find outdated pod to delete")
		return err
	}
	if err := r.GetClient().Delete(ctx, pod); err != nil {
		log.Error(err, "unable to delete outdated pod", "pod", pod)
		return err
	}
	log.V(1).Info("Pod deleted", "pod", pod)
	return nil
}

func (r DeviceReconciler) createPod(
	device *networksimulatorv1.Device, ctx context.Context, log logr.Logger) (*v1.Pod, error) {
	name := device.Name + "-pod"
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        name,
			Namespace:   device.Spec.NetworkName,
		},
		Spec: *device.Spec.PodTemplate.Spec.DeepCopy(),
	}
	if err := ctrl.SetControllerReference(device, pod, r.Scheme); err != nil {
		log.V(1).Info("Failed to set controller reference for pod", "pod", pod)
		return nil, err
	}

	if err := r.GetClient().Create(ctx, pod); err != nil {
		log.Error(err, "unable to create Pod for device", "pod", pod)
		return nil, err
	}
	log.V(1).Info("Created pod")

	return pod, nil
}

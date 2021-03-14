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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

// ManageDeviceServiceLogic manages service logic for given device
func (r DeviceReconciler) ManageDeviceServiceLogic(device networksimulatorv1.Device, ctx context.Context, log logr.Logger) (ctrl.Result, error, bool) {
	if r.isServiceBeingDeleted(&device, ctx) {
		log.V(1).Info("Service is being deleted")
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil, false
	}

	if err := r.createOrUpdateService(&device, ctx, log); err != nil {
		return ctrl.Result{}, err, false
	}
	return ctrl.Result{}, nil, true
}

// isServiceBeingDeleted checks if the service is being deleted
func (r DeviceReconciler) isServiceBeingDeleted(device *networksimulatorv1.Device, ctx context.Context) bool {
	service, err := r.getService(device, ctx)
	if err != nil {
		return false
	}
	return util.IsBeingDeleted(service)
}

// ManageCleanUpService cleans up services created for the device
func (r DeviceReconciler) ManageCleanUpService(device networksimulatorv1.Device, ctx context.Context, log logr.Logger) error {
	service, err := r.getService(&device, ctx)
	if err != nil {
		// Service doesnt exist
		return nil
	}
	if err := r.GetClient().Delete(ctx, service); err != nil {
		log.Error(err, "unable to delete service when cleaning up", "service", service)
		return err
	}
	log.V(1).Info("Deleted service when cleaning up")
	return nil
}

// getService returns service for given device
func (r DeviceReconciler) getService(device *networksimulatorv1.Device, ctx context.Context) (*v1.Service, error) {
	var service v1.Service
	err := r.GetClient().Get(
		ctx,
		types.NamespacedName{
			Namespace: device.Spec.NetworkName,
			Name:      device.ServiceName(),
		},
		&service)
	if err != nil {
		return nil, err
	}
	return &service, nil
}

// createOrUpdateService creates or updates the service
func (r DeviceReconciler) createOrUpdateService(device *networksimulatorv1.Device, ctx context.Context, log logr.Logger) error {
	name := device.ServiceName()
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        name,
			Namespace:   device.Spec.NetworkName,
		},
		Spec: *device.Spec.ServiceSpec.DeepCopy(),
	}
	service.ObjectMeta.Labels["Patriot-Device"] = device.Name
	service.ObjectMeta.Labels["Patriot"] = "device"

	if err := ctrl.SetControllerReference(device, service, r.Scheme); err != nil {
		log.V(1).Info("Failed to set controller reference for service", "service", service)
		return err
	}

	op, err := ctrl.CreateOrUpdate(ctx, r.GetClient(), service, func() error {
		if service.Spec.Selector == nil {
			service.Spec.Selector = make(map[string]string)
		}
		if len(service.Spec.Selector) == 0 {
			service.Spec.Selector["Patriot-Device"] = device.PodName()
		}

		return nil
	})
	if err != nil {
		log.Error(err, "create or update service failed")
		return err
	}
	log.V(1).Info("Created or Updated the service successfully", "operation", op)
	return nil
}

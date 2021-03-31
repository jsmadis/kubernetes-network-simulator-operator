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
	v12 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ManageNetworkPolicyLogic manages all logic for device controller about network policy, creating, updating deleting...
func (r DeviceReconciler) ManageNetworkPolicyLogic(device networksimulatorv1.Device, ctx context.Context, log logr.Logger) (ctrl.Result, error, bool) {
	if r.shouldBeNetworkPolicyConnectionCreated(device, ctx) {
		if err := r.manageNetworkPolicyConnection(&device, ctx, log); err != nil {
			return ctrl.Result{}, err, false
		}
		return ctrl.Result{}, nil, false
	} else {
		if r.isNetworkPolicyCreated(device.NetworkNameConnection(), device, ctx) {
			if err := r.DeleteNetworkPolicy(device.NetworkNameConnection(), device.Spec.NetworkName, ctx, log); err != nil {
				return ctrl.Result{}, nil, false
			}
		}
	}

	if err := r.manageNetworkPolicyDefault(&device, ctx, log); err != nil {
		return ctrl.Result{}, nil, false
	}

	return ctrl.Result{}, nil, true
}

// ManageCleanUpNetworkPolicy manages clean up of all network policies created for given device
func (r DeviceReconciler) ManageCleanUpNetworkPolicy(device networksimulatorv1.Device, ctx context.Context, log logr.Logger) error {
	if r.isNetworkPolicyCreated(device.NetworkNameConnection(), device, ctx) {
		if err := r.DeleteNetworkPolicy(device.NetworkNameConnection(), device.Spec.NetworkName, ctx, log); err != nil {
			return err
		}
	}

	if r.isNetworkPolicyCreated(device.NetworkNameDefault(), device, ctx) {
		if err := r.DeleteNetworkPolicy(device.NetworkNameDefault(), device.Spec.NetworkName, ctx, log); err != nil {
			return err
		}
	}
	return nil
}

// isNetworkPolicyCreated checks if the network policy is created for given device
func (r DeviceReconciler) isNetworkPolicyCreated(name string, device networksimulatorv1.Device, ctx context.Context) bool {
	networkPolicy, err := r.GetNetworkPolicy(name, device.Spec.NetworkName, ctx)
	if err != nil {
		return false
	}
	return networkPolicy.Name == name
}

// shouldBeNetworkPolicyCreated checks if it is necessary to create network policy for given device
func (r DeviceReconciler) shouldBeNetworkPolicyConnectionCreated(device networksimulatorv1.Device, ctx context.Context) bool {
	return !(len(device.Spec.DeviceEgressPorts) == 0 && len(device.Spec.DeviceIngressPorts) == 0)
}

// manageNetworkPolicyDeviceConnection creates or updates network policy for device connection
// with other devices outside the network
func (r DeviceReconciler) manageNetworkPolicyConnection(
	device *networksimulatorv1.Device, ctx context.Context, log logr.Logger) error {
	name := device.NetworkNameConnection()

	ingress := util.ProcessIngressNetworkPolicy(device.Spec.DeviceIngressPorts)
	egress := util.ProcessEgressNetworkPolicy(device.Spec.DeviceEgressPorts)

	networkPolicy := &v12.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        name,
			Namespace:   device.Spec.NetworkName,
		},
		Spec: v12.NetworkPolicySpec{
			PodSelector: devicePodSelector(device),
			Ingress:     ingress,
			Egress:      egress,
			PolicyTypes: []v12.PolicyType{v12.PolicyTypeEgress, v12.PolicyTypeIngress},
		},
	}

	return r.createOrUpdateNetworkPolicy(networkPolicy, device, ctx, log)
}

// manageNetworkPolicyDefault manages default network policy that is created from device.Spec.NetworkPolicySpec
func (r DeviceReconciler) manageNetworkPolicyDefault(device *networksimulatorv1.Device, ctx context.Context, log logr.Logger) error {
	// Skip when NetworkPolicySpec is empty
	if reflect.DeepEqual(device.Spec.NetworkPolicySpec, v12.NetworkPolicySpec{}) {
		return nil
	}

	name := device.NetworkNameDefault()

	networkPolicy := &v12.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        name,
			Namespace:   device.Spec.NetworkName,
		},
		Spec: *device.Spec.NetworkPolicySpec.DeepCopy(),
	}
	networkPolicy.Spec.PodSelector = devicePodSelector(device)

	return r.createOrUpdateNetworkPolicy(networkPolicy, device, ctx, log)
}

// createOrUpdateNetworkPolicy creates or updates network policy for the device
func (r DeviceReconciler) createOrUpdateNetworkPolicy(policy *v12.NetworkPolicy,
	device *networksimulatorv1.Device, ctx context.Context, log logr.Logger) error {

	if err := ctrl.SetControllerReference(device, policy, r.Scheme); err != nil {
		log.Error(err, "unable to set controller reference to the network policy")
		return err
	}

	op, err := ctrl.CreateOrUpdate(ctx, r.GetClient(), policy, func() error { return nil })
	if err != nil {
		log.Error(err, "Unable to create or update network policy for device", "network-policy", policy)
		return err
	}

	log.V(1).Info("Created network policy", "network-policy", policy, "operation", op)

	return nil
}

// devicePodSelector returns pod selector that selects given device
func devicePodSelector(device *networksimulatorv1.Device) metav1.LabelSelector {
	return metav1.LabelSelector{
		MatchLabels: map[string]string{"Patriot-Device": device.Name},
	}
}

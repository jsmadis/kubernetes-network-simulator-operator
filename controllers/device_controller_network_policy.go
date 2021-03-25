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
			if err := r.deleteNetworkPolicy(device.NetworkNameConnection(), device, ctx, log); err != nil {
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
		if err := r.deleteNetworkPolicy(device.NetworkNameConnection(), device, ctx, log); err != nil {
			return err
		}
	}

	if r.isNetworkPolicyCreated(device.NetworkNameDefault(), device, ctx) {
		if err := r.deleteNetworkPolicy(device.NetworkNameDefault(), device, ctx, log); err != nil {
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
	if len(device.Spec.DeviceEgressPorts) == 0 && len(device.Spec.DeviceIngressPorts) == 0 {
		return false
	}
	return !r.isNetworkPolicyCreated(device.NetworkNameConnection(), device, ctx)
}

// deleteNetworkPolicy deletes network policy created for given device
func (r DeviceReconciler) deleteNetworkPolicy(name string, device networksimulatorv1.Device, ctx context.Context, log logr.Logger) error {
	networkPolicy, err := r.GetNetworkPolicy(name, device.Spec.NetworkName, ctx)
	if err != nil {
		log.V(1).Info("Unable to get network policy when cleaning up", "err", err)
		return err
	}
	if err := r.GetClient().Delete(ctx, networkPolicy); err != nil {
		log.Error(err, "unable to delete network policy for device when cleaning up")
		return err
	}
	log.V(1).Info("Deleted network policy for the device", "network-policy", networkPolicy)
	return nil

}

// manageNetworkPolicyDeviceConnection creates or updates network policy for device connection
// with other devices outside the network
func (r DeviceReconciler) manageNetworkPolicyConnection(
	device *networksimulatorv1.Device, ctx context.Context, log logr.Logger) error {
	name := device.NetworkNameConnection()

	ingress := processIngressNetworkPolicy(device)
	egress := processEgressNetworkPolicy(device)

	networkPolicy := &v12.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        name,
			Namespace:   device.Spec.NetworkName,
		},
		Spec: v12.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"Patriot-Device": device.Name},
			},
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

// processIngressNetworkPolicy
// returns array of network policy ingress rules that are created from device.Spec.DeviceIngressPorts
func processIngressNetworkPolicy(device *networksimulatorv1.Device) []v12.NetworkPolicyIngressRule {
	var ingress []v12.NetworkPolicyIngressRule
	for _, deviceIngressPort := range device.Spec.DeviceIngressPorts {
		ingress = append(ingress, v12.NetworkPolicyIngressRule{
			Ports: deviceIngressPort.NetworkPolicyPorts,
			From:  processNetworkPolicyPeer(deviceIngressPort),
		})
	}
	return ingress
}

// processEgressNetworkPolicy
//returns array of network policy egress rules that are created from device.Spec.DeviceEgressPorts
func processEgressNetworkPolicy(device *networksimulatorv1.Device) []v12.NetworkPolicyEgressRule {
	var egress []v12.NetworkPolicyEgressRule
	for _, deviceEgressPort := range device.Spec.DeviceEgressPorts {
		egress = append(egress, v12.NetworkPolicyEgressRule{
			Ports: deviceEgressPort.NetworkPolicyPorts,
			To:    processNetworkPolicyPeer(deviceEgressPort),
		})
	}
	return egress
}

//processNetworkPolicyPeer creates array of network policy pear from device port struct
func processNetworkPolicyPeer(ports networksimulatorv1.DevicePorts) []v12.NetworkPolicyPeer {
	var peers []v12.NetworkPolicyPeer

	// If DeviceName is empty select every pod --> device should see every pod from other network
	podSelectorLabelSelector := &metav1.LabelSelector{}
	if ports.DeviceName != "" {
		podSelectorLabelSelector.MatchLabels = map[string]string{"Patriot-Device": ports.DeviceName}
	}

	peers = append(peers, v12.NetworkPolicyPeer{
		PodSelector: podSelectorLabelSelector,
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"Patriot-Network": ports.NetworkName},
		},
	})
	return peers
}

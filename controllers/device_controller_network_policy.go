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
	ctrl "sigs.k8s.io/controller-runtime"
)

// ManageNetworkPolicyLogic manages all logic for device controller about network policy, creating, updating deleting...
func (r DeviceReconciler) ManageNetworkPolicyLogic(device networksimulatorv1.Device, ctx context.Context, log logr.Logger) (ctrl.Result, error, bool) {
	if r.shouldBeNetworkPolicyCreated(device, ctx) {
		if err := r.createNetworkPolicy(&device, ctx, log); err != nil {
			return ctrl.Result{}, err, false
		}
		return ctrl.Result{}, nil, false
	}
	return ctrl.Result{}, nil, true
}

// ManageCleanUpNetworkPolicy manages clean up of all network policies created for given device
func (r DeviceReconciler) ManageCleanUpNetworkPolicy(device networksimulatorv1.Device, ctx context.Context, log logr.Logger) error {
	if r.isNetworkPolicyCreated(device, ctx) {
		if err := r.deleteNetworkPolicy(device, ctx, log); err != nil {
			return err
		}
	}
	return nil
}

// isNetworkPolicyCreated checks if the network policy is created for given device
func (r DeviceReconciler) isNetworkPolicyCreated(device networksimulatorv1.Device, ctx context.Context) bool {
	networkPolicy, err := r.GetNetworkPolicy(device.Name, device.Spec.NetworkName, ctx)
	if err != nil {
		return false
	}
	return networkPolicy.Name == device.Name+"-network-policy"
}

// shouldBeNetworkPolicyCreated checks if it is necessary to create network policy for given device
func (r DeviceReconciler) shouldBeNetworkPolicyCreated(device networksimulatorv1.Device, ctx context.Context) bool {
	if len(device.Spec.DeviceEgressPorts) == 0 && len(device.Spec.DeviceIngressPorts) == 0 {
		return false
	}
	return !r.isNetworkPolicyCreated(device, ctx)
}

// deleteNetworkPolicy deletes network policy created for given device
func (r DeviceReconciler) deleteNetworkPolicy(device networksimulatorv1.Device, ctx context.Context, log logr.Logger) error {
	networkPolicy, err := r.GetNetworkPolicy(device.Name, device.Spec.NetworkName, ctx)
	if err != nil {
		log.V(1).Info("Unable to get network policy when cleaning up", "err", err)
		return err
	}
	if err := r.GetClient().Delete(ctx, networkPolicy); err != nil {
		log.Error(err, "unable to delete network policy for device when cleaning up")
		return err
	}
	log.V(1).Info("Network policy for the device successfully deleted", "network-policy", networkPolicy)
	return nil

}

// createNetworkPolicy creates network policy for given device
func (r DeviceReconciler) createNetworkPolicy(
	device *networksimulatorv1.Device, ctx context.Context, log logr.Logger) error {
	name := device.Name + "-network-policy"

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

	if err := ctrl.SetControllerReference(device, networkPolicy, r.Scheme); err != nil {
		log.Error(err, "unable to set controller reference to the network policy")
		return err
	}

	if err := r.GetClient().Create(ctx, networkPolicy); err != nil {
		log.Error(err, "unable to create network policy for device", "device", device)
		return err
	}
	log.V(1).Info("Created network policy for device", "device", device, "network-policy", networkPolicy)

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

	peers = append(peers, v12.NetworkPolicyPeer{
		PodSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"Patriot-Device": ports.DeviceName},
		},
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"Patriot-Network": ports.NetworkName},
		},
	})
	return peers
}

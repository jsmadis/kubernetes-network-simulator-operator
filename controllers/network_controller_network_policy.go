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
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

// ManageNetworkPolicyLogic manages whole logic about network policies for the patriot network
func (r NetworkReconciler) ManageNetworkPolicyLogic(network networksimulatorv1.Network, ctx context.Context, log logr.Logger) (ctrl.Result, error, bool) {
	if r.isNetworkPolicyBeingDeleted(&network, ctx) {
		log.V(1).Info("Network policy is being deleted")
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil, false
	}

	if !r.deleteOutdatedNetworkPolicy(&network, ctx, log) {
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil, false
	}

	if !r.isNetworkPolicyCreated(network, ctx) {
		if err := r.createNetworkPolicy(&network, ctx, log); err != nil {
			return ctrl.Result{}, err, false
		}
		return ctrl.Result{}, nil, false
	}
	return ctrl.Result{}, nil, true
}

// isNetworkPolicyCreated checks if the network policy for the patriot network is created
func (r NetworkReconciler) isNetworkPolicyCreated(network networksimulatorv1.Network, ctx context.Context) bool {
	networkPolicy, err := r.GetNetworkPolicy(network.NetworkName(), network.Spec.Name, ctx)
	if err != nil {
		return false
	}
	return networkPolicy.Name == network.NetworkName()
}

// isNetworkPolicyBeingDeleted checks if the network policy for the patrit network is being deleted
func (r NetworkReconciler) isNetworkPolicyBeingDeleted(network *networksimulatorv1.Network, ctx context.Context) bool {
	networkPolicy, err := r.GetNetworkPolicy(network.Spec.Name, network.Spec.Name, ctx)
	if err != nil {
		return false
	}
	return util.IsBeingDeleted(networkPolicy)
}

// deleteOutdatedNetworkPolicy removes outdated network policy
// TODO: Maybe we should change it to update?
func (r NetworkReconciler) deleteOutdatedNetworkPolicy(
	network *networksimulatorv1.Network, ctx context.Context, log logr.Logger) bool {
	if network.Spec.AllowIngressTraffic == network.Status.AllowIngressTraffic &&
		network.Spec.AllowEgressTraffic == network.Status.AllowEgressTraffic {
		return true
	}
	networkPolicy, err := r.GetNetworkPolicy(network.Spec.Name, network.Spec.Name, ctx)
	if err != nil {
		log.V(1).Info("Expected network policy not found", "err", err)
		// Network policy already deleted
		network.Status.AllowEgressTraffic = network.Spec.AllowEgressTraffic
		network.Status.AllowIngressTraffic = network.Spec.AllowIngressTraffic
		if err := r.updateNetworkStatus(network, ctx, log); err != nil {
			log.Error(err, "unable to update network status in deleteOutdatedNetworkPolicy")
			return false
		}
		return false
	}

	if err := r.GetClient().Delete(ctx, networkPolicy); err != nil {
		log.Error(err, "unable to delete outdated network policy")
		return false
	}

	log.V(1).Info("Deleted outdated network policy", "networkPolicy", networkPolicy)
	return false
}

// createNetworkPolicy creates netwokr policy for the patriot network
func (r *NetworkReconciler) createNetworkPolicy(network *networksimulatorv1.Network, ctx context.Context, log logr.Logger) error {
	namespace, err := r.GetNamespace(network.Spec.Name, ctx)
	if err != nil {
		log.Error(err, "unable to get namespace for network", "network", network)
		return err
	}

	name := network.NetworkName()
	var ingress []v12.NetworkPolicyIngressRule
	var egress []v12.NetworkPolicyEgressRule

	if network.Spec.AllowEgressTraffic {
		egress = append(egress, v12.NetworkPolicyEgressRule{
			Ports: nil,
			To:    []v12.NetworkPolicyPeer{networkPolicyPeerLocalNamespace(network)},
		})
	}
	if network.Spec.AllowIngressTraffic {
		ingress = append(ingress, v12.NetworkPolicyIngressRule{
			Ports: nil,
			From:  []v12.NetworkPolicyPeer{networkPolicyPeerLocalNamespace(network)},
		})
	}

	networkPolicy := &v12.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        name,
			Namespace:   namespace.Name,
		},
		Spec: v12.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			Ingress:     ingress,
			Egress:      egress,
			PolicyTypes: []v12.PolicyType{v12.PolicyTypeEgress, v12.PolicyTypeIngress},
		},
	}
	if err := ctrl.SetControllerReference(network, networkPolicy, r.Scheme); err != nil {
		log.Error(err, "Unable to set controller reference to network policy")
		return err
	}

	if err := r.GetClient().Create(ctx, networkPolicy); err != nil {
		log.Error(err, "Unable to create network policy for network", "network-policy", networkPolicy)
		return err
	}

	log.V(1).Info("Created network policy", "network-policy", networkPolicy)

	network.Status.AllowEgressTraffic = network.Spec.AllowEgressTraffic
	network.Status.AllowIngressTraffic = network.Spec.AllowIngressTraffic
	if err := r.updateNetworkStatus(network, ctx, log); err != nil {
		return err
	}
	return nil
}

func networkPolicyPeerLocalNamespace(network *networksimulatorv1.Network) v12.NetworkPolicyPeer {
	return v12.NetworkPolicyPeer{
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"Patriot-Network": network.Spec.Name},
		},
	}
}

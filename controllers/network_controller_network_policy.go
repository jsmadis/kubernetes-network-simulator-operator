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
	if r.isNetworkPolicyBeingDeleted(network.NetworkPolicyNameIsolation(), &network, ctx) {
		log.V(1).Info("Network policy is being deleted")
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil, false
	}

	if r.isNetworkPolicyBeingDeleted(network.NetworkPolicyNameInternet(), &network, ctx) {
		log.V(1).Info("Internet network policy is being deleted")
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil, false
	}

	if err := r.manageIsolationNetworkPolicy(&network, ctx, log); err != nil {
		return ctrl.Result{}, err, false
	}

	if err := r.manageInternetNetworkPolicy(&network, ctx, log); err != nil {
		return ctrl.Result{}, err, false
	}

	return ctrl.Result{}, nil, true
}

// isNetworkPolicyBeingDeleted checks if the network policy for the patrit network is being deleted
func (r NetworkReconciler) isNetworkPolicyBeingDeleted(name string, network *networksimulatorv1.Network, ctx context.Context) bool {
	networkPolicy, err := r.GetNetworkPolicy(name, network.Name, ctx)
	if err != nil {
		return false
	}
	return util.IsBeingDeleted(networkPolicy)
}

// manageIsolationNetworkPolicy manages the network policy which isolates the network (creates or updates the netwrok policy)
func (r NetworkReconciler) manageIsolationNetworkPolicy(network *networksimulatorv1.Network, ctx context.Context, log logr.Logger) error {
	name := network.NetworkPolicyNameIsolation()
	var ingress []v12.NetworkPolicyIngressRule
	var egress []v12.NetworkPolicyEgressRule

	if !network.Spec.DisableInsideEgressTraffic {
		egress = append(egress, v12.NetworkPolicyEgressRule{
			Ports: nil,
			To:    []v12.NetworkPolicyPeer{networkPolicyPeerLocalNamespace(network)},
		})
	}
	if !network.Spec.DisableInsideIngressTraffic {
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
			Namespace:   network.Name,
		},
		Spec: v12.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			Ingress:     ingress,
			Egress:      egress,
			PolicyTypes: []v12.PolicyType{v12.PolicyTypeEgress, v12.PolicyTypeIngress},
		},
	}
	return r.createOrUpdateNetworkPolicy(networkPolicy, network, ctx, log)
}

// manageInternetNetworkPolicy manages network policy that enables internet (egress) network policy
// that enables for selected pods (with label "Patriot-allow-internet": "true") to be able to connect to the internet
func (r NetworkReconciler) manageInternetNetworkPolicy(network *networksimulatorv1.Network, ctx context.Context, log logr.Logger) error {
	name := network.NetworkPolicyNameInternet()

	networkPolicy := &v12.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        name,
			Namespace:   network.Name,
		},
		Spec: v12.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"Patriot-allow-internet": "true"},
			},
			Egress:      []v12.NetworkPolicyEgressRule{},
			PolicyTypes: []v12.PolicyType{v12.PolicyTypeEgress},
		},
	}

	return r.createOrUpdateNetworkPolicy(networkPolicy, network, ctx, log)
}

// createNetworkPolicy creates or updates network policy for the patriot network
func (r *NetworkReconciler) createOrUpdateNetworkPolicy(networkPolicy *v12.NetworkPolicy,
	network *networksimulatorv1.Network, ctx context.Context, log logr.Logger) error {

	if err := ctrl.SetControllerReference(network, networkPolicy, r.Scheme); err != nil {
		log.Error(err, "Unable to set controller reference to network policy")
		return err
	}

	op, err := ctrl.CreateOrUpdate(ctx, r.GetClient(), networkPolicy, func() error { return nil })
	if err != nil {
		log.Error(err, "Unable to create or update network policy for network", "network-policy", networkPolicy)
		return err
	}

	log.V(1).Info("Created network policy", "network-policy", networkPolicy, "operation", op)

	return nil
}

// networkPolicyPeerLocalNamespace returns network policy peer with namespace selector to the patriot network
func networkPolicyPeerLocalNamespace(network *networksimulatorv1.Network) v12.NetworkPolicyPeer {
	return v12.NetworkPolicyPeer{
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"Patriot-Network": network.Name},
		},
	}
}

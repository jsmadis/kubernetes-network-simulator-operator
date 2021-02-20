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
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networksimulatorv1 "github.com/jsmadis/kubernetes-network-simulator-operator/api/v1"
)

// NetworkReconciler reconciles a Network object
type NetworkReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=network-simulator.patriot-framework.io,resources=networks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=network-simulator.patriot-framework.io,resources=networks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=network-simulator.patriot-framework.io,resources=networks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Network object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *NetworkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("network", req.NamespacedName)

	var network networksimulatorv1.Network
	if err := r.Get(ctx, req.NamespacedName, &network); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	createNameSpace := func(network *networksimulatorv1.Network) (*v1.Namespace, error) {
		namespace := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				Name:        network.Name,
			},
			Spec:   v1.NamespaceSpec{},
			Status: v1.NamespaceStatus{},
		}
		if err := ctrl.SetControllerReference(network, namespace, r.Scheme); err != nil {
			return nil, err
		}
		return namespace, nil
	}

	createNetworkPolicy := func(network *networksimulatorv1.Network, namespace *v1.Namespace) (*v12.NetworkPolicy, error) {
		name := network.Spec.Name + "-network-policy"
		var ingress []v12.NetworkPolicyIngressRule
		var egress []v12.NetworkPolicyEgressRule

		log.V(1).Info("Ingress rule:", "ingress", ingress)
		log.V(1).Info("Egress rule:", "egress", egress)

		if network.Spec.AllowEgressTraffic {
			egress = append(egress, v12.NetworkPolicyEgressRule{
				Ports: nil,
				To:    nil,
			})
		}
		if network.Spec.AllowIngressTraffic {
			ingress = append(ingress, v12.NetworkPolicyIngressRule{
				Ports: nil,
				From:  nil,
			})
		}
		log.V(1).Info("Ingress rule:", "ingress", ingress)
		log.V(1).Info("Egress rule:", "egress", egress)
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
			return nil, err
		}
		return networkPolicy, nil
	}

	namespace, err := createNameSpace(&network)
	if err != nil {
		log.V(1).Info("Failed to create namespace")
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, namespace); err != nil {
		log.Error(err, "Unable to create Namespace for network", "namespace", namespace)
		return ctrl.Result{}, err
	}
	log.V(1).Info("Created namespace", "namespace", namespace.Name)

	networkPolicy, err := createNetworkPolicy(&network, namespace)

	if err != nil {
		log.V(1).Info("Failed to create network policy")
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, networkPolicy); err != nil {
		log.Error(err, "Unable to create network policy for network", "network-policy", networkPolicy)
		return ctrl.Result{}, err
	}

	log.V(1).Info("Created network policy", "network-policy", networkPolicy)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networksimulatorv1.Network{}).
		Complete(r)
}

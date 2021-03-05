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
	v12 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// NetworkReconciler reconciles a Network object
type NetworkReconciler struct {
	util.ReconcilerBase
}

const controllerName = "NetworkCRD"

//+kubebuilder:rbac:groups=network-simulator.patriot-framework.io,resources=networks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=network-simulator.patriot-framework.io,resources=networks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=network-simulator.patriot-framework.io,resources=networks/finalizers,verbs=update
//+kubebuilder:rbac:groups=network-simulator.patriot-framework.io,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=network-simulator.patriot-framework.io,resources=namespaces/status,verbs=get
//+kubebuilder:rbac:groups=network-simulator.patriot-framework.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete

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
	if err := r.GetClient().Get(ctx, req.NamespacedName, &network); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if ok, err := r.IsValid(&network); !ok {
		log.Error(err, "Invalid CR of network", "network", "CR", network)
		return ctrl.Result{}, err
	}

	if ok := r.IsInitialized(&network); !ok {
		err := r.GetClient().Update(ctx, &network)
		if err != nil {
			log.Error(err, "unable to update instance", "network", network)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if util.IsBeingDeleted(&network) {
		if !util.HasFinalizer(&network, controllerName) {
			return ctrl.Result{}, nil
		}
		err := r.ManageCleanUpLogic(network, ctx, log)
		if err != nil {
			log.Error(err, "unable to delete network", "network", network)
			return ctrl.Result{}, err
		}
		util.RemoveFinalizer(&network, controllerName)
		err = r.GetClient().Update(ctx, &network)
		if err != nil {
			log.Error(err, "unable to update network", "network", network)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil

	}
	return r.ManageOperatorLogic(network, ctx, log)
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&networksimulatorv1.Network{}).
		Complete(r)
	if err != nil {
		return err
	}
	return r.addWatcher(mgr)
}

// addWatcher adds watcher for resources created by network controller
func (r NetworkReconciler) addWatcher(mgr ctrl.Manager) error {
	c, err := controller.New("network-controller", mgr, controller.Options{Reconciler: &r})
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

	// Watch for namespaces owned by Network
	err = c.Watch(&source.Kind{Type: &v1.Namespace{}}, &handler.EnqueueRequestForOwner{
		OwnerType:    &networksimulatorv1.Network{},
		IsController: false,
	},
		p)
	if err != nil {
		return err
	}

	// Watch for network policies owned by Network
	err = c.Watch(&source.Kind{Type: &v12.NetworkPolicy{}}, &handler.EnqueueRequestForOwner{
		OwnerType:    &networksimulatorv1.Network{},
		IsController: false,
	},
		p)
	if err != nil {
		return err
	}
	return nil
}

func (r *NetworkReconciler) IsInitialized(obj metav1.Object) bool {
	networkCrd, ok := obj.(*networksimulatorv1.Network)
	if !ok {
		return false
	}
	if util.HasFinalizer(networkCrd, controllerName) {
		return true
	}
	util.AddFinalizer(networkCrd, controllerName)
	return false
}

func (r NetworkReconciler) IsNamespaceCreated(network networksimulatorv1.Network, ctx context.Context) bool {
	namespace, err := r.GetNamespace(network.Spec.Name, ctx)
	if err != nil {
		return false
	}
	return namespace.Name == network.Spec.Name
}

func (r NetworkReconciler) IsNetworkPolicyCreated(network networksimulatorv1.Network, ctx context.Context) bool {
	namespacedName := types.NamespacedName{
		Namespace: network.Spec.Name,
		Name:      network.Spec.Name + "-network-policy",
	}

	var networkPolicy v12.NetworkPolicy
	if err := r.GetClient().Get(ctx, namespacedName, &networkPolicy); err != nil {
		return false
	}
	return networkPolicy.Name == network.Spec.Name+"-network-policy"
}

func (r NetworkReconciler) ManageOperatorLogic(
	network networksimulatorv1.Network, ctx context.Context, log logr.Logger) (ctrl.Result, error) {

	// requeue reconcilation when namespace is being deleted
	if r.IsNamespaceBeingDeleted(network.Spec.Name, ctx) {
		return ctrl.Result{RequeueAfter: 2}, nil
	}

	// is namespace created? --> create everything
	if !r.IsNamespaceCreated(network, ctx) {
		namespace, err := r.createNamespace(&network, ctx, log)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = r.createNetworkPolicy(&network, namespace, ctx, log)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	//is network policy created? --> create only network policy
	if !r.IsNetworkPolicyCreated(network, ctx) {
		namespace, err := r.GetNamespace(network.Spec.Name, ctx)
		if err != nil {
			log.Error(err, "unable to get namespace for network", "network", network)
			return ctrl.Result{}, err
		}

		err = r.createNetworkPolicy(&network, namespace, ctx, log)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r NetworkReconciler) ManageCleanUpLogic(network networksimulatorv1.Network,
	ctx context.Context, log logr.Logger) error {

	namespace, err := r.GetNamespace(network.Spec.Name, ctx)
	if err != nil {
		// namespace doesn't exist, we don't need to to anything
		return nil
	}
	if err := r.GetClient().Delete(ctx, namespace); err != nil {
		log.Error(err, "unable to delete namespace for network when cleaning up",
			"namespace", namespace)
		return err
	}
	return nil
}

func (r *NetworkReconciler) createNamespace(
	network *networksimulatorv1.Network, ctx context.Context, log logr.Logger) (*v1.Namespace, error) {
	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        network.Spec.Name,
		},
		Spec:   v1.NamespaceSpec{},
		Status: v1.NamespaceStatus{},
	}
	if err := ctrl.SetControllerReference(network, namespace, r.Scheme); err != nil {
		log.Error(err, "Unable to set controller reference to namespace")
		return nil, err
	}
	if err := r.GetClient().Create(ctx, namespace); err != nil {
		log.Error(err, "Unable to create Namespace for network", "namespace", namespace)
		return nil, err
	}
	log.V(1).Info("Created namespace", "namespace", namespace)
	return namespace, nil
}

func (r *NetworkReconciler) createNetworkPolicy(network *networksimulatorv1.Network, namespace *v1.Namespace,
	ctx context.Context, log logr.Logger) error {
	name := network.Spec.Name + "-network-policy"
	var ingress []v12.NetworkPolicyIngressRule
	var egress []v12.NetworkPolicyEgressRule

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
	return nil
}

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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

// ManageNamespaceLogic manage all logic about namespace for network
func (r NetworkReconciler) ManageNamespaceLogic(network networksimulatorv1.Network, ctx context.Context, log logr.Logger) (ctrl.Result, error, bool) {
	// requeue reconcilation when namespace is being deleted
	if r.IsNamespaceBeingDeleted(network.Spec.Name, ctx) {
		log.V(1).Info("Namespace is being deleted")
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil, false
	}
	if !r.deleteOldNamespace(&network, ctx, log) {
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil, false
	}

	if !r.IsNamespaceCreated(network, ctx) {
		namespace, err := r.createNamespace(&network, ctx, log)
		if err != nil {
			return ctrl.Result{}, err, false
		}
		err = r.createNetworkPolicy(&network, namespace, ctx, log)
		if err != nil {
			return ctrl.Result{}, err, false
		}
		return ctrl.Result{}, nil, false
	}
	return ctrl.Result{}, nil, true
}

// ManageCleanUpNamespace deletes the namespace, no need to delete anything else since we creates resources inside namespace
func (r NetworkReconciler) ManageCleanUpNamespace(network networksimulatorv1.Network, ctx context.Context, log logr.Logger) error {
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

// IsNamespaceCreated checks if the namespace for network is created
func (r NetworkReconciler) IsNamespaceCreated(network networksimulatorv1.Network, ctx context.Context) bool {
	namespace, err := r.GetNamespace(network.Spec.Name, ctx)
	if err != nil {
		return false
	}
	return namespace.Name == network.Spec.Name
}

// deleteOldNamespace delete old namespace when the CRD is updated with new name of the namespace
func (r NetworkReconciler) deleteOldNamespace(
	network *networksimulatorv1.Network, ctx context.Context, log logr.Logger) bool {
	if network.Spec.Name == network.Status.Name {
		return true
	}
	if network.Status.Name == "" {
		return true
	}
	namespace, err := r.GetNamespace(network.Status.Name, ctx)
	if err != nil {
		log.V(1).Info("Expected namespace not found", "err", err)
		// Namespace is already deleted
		network.Status.Name = network.Spec.Name
		if err := r.updateNetworkStatus(network, ctx, log); err != nil {
			log.Error(err, "unable to update status with correct network name")
			return false
		}
		return false
	}

	if err := r.GetClient().Delete(ctx, namespace); err != nil {
		log.Error(err, "unable to delete forgotten old namespace", "namespace", namespace)
		return false
	}

	network.Status.Name = network.Spec.Name
	if err := r.updateNetworkStatus(network, ctx, log); err != nil {
		log.Error(err, "unable to update status with correct network name after old namespace was deleted")
		return false
	}

	log.V(1).Info("Old namespace deleted", "namespace", namespace)
	return false
}

// createNamespace creates namespace for the network
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
	namespace.ObjectMeta.Labels["Patriot-Network"] = network.Spec.Name

	if err := r.GetClient().Create(ctx, namespace); err != nil {
		log.Error(err, "Unable to create Namespace for network", "namespace", namespace)
		return nil, err
	}
	log.V(1).Info("Created namespace", "namespace", namespace)

	network.Status.Name = network.Spec.Name
	if err := r.updateNetworkStatus(network, ctx, log); err != nil {
		return nil, err
	}
	return namespace, nil
}
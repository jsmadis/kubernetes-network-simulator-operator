package util

import (
	"context"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ReconcilerBase struct {
	Client client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func NewRencolicerBase(client client.Client, log logr.Logger, scheme *runtime.Scheme) ReconcilerBase {
	return ReconcilerBase{
		Client: client,
		Log:    log,
		Scheme: scheme,
	}
}

func (r *ReconcilerBase) IsValid(obj metav1.Object) (bool, error) {
	return true, nil
}

func (r *ReconcilerBase) IsInitialized(obj metav1.Object) bool {
	return true
}

func (r *ReconcilerBase) Reconcile(ctx context.Context, req ctrl.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func (r *ReconcilerBase) GetClient() client.Client {
	return r.Client
}

func (r *ReconcilerBase) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// GetNamespace returns namespace for given name
func (r ReconcilerBase) GetNamespace(name string, ctx context.Context) (*v1.Namespace, error) {
	namespacedName := types.NamespacedName{
		Name: name,
	}

	var namespace v1.Namespace
	if err := r.GetClient().Get(ctx, namespacedName, &namespace); err != nil {
		return nil, err
	}
	return &namespace, nil
}

// IsNamespaceBeingDeleted checks if the namespace is being deleted
func (r ReconcilerBase) IsNamespaceBeingDeleted(name string, ctx context.Context) bool {
	namespace, err := r.GetNamespace(name, ctx)
	if err != nil {
		return false
	}
	return IsBeingDeleted(namespace)
}

// GetNetworkPolicy returns network policy for given name and namespace
// We are adding '-network-policy' suffix to the network policy name
func (r ReconcilerBase) GetNetworkPolicy(name string, namespace string, ctx context.Context) (*v12.NetworkPolicy, error) {
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	var networkPolicy v12.NetworkPolicy
	if err := r.GetClient().Get(ctx, namespacedName, &networkPolicy); err != nil {
		return nil, err
	}
	return &networkPolicy, nil

}

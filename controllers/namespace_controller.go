/*
Copyright 2022.

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

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//TODO: Read value(s) from configmap?
const (
	namespaceCapabilityNameLabel = "capability-name"
)

// NamespaceReconciler reconciles a Namespace object
type NamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=namespaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=namespaces/finalizers,verbs=update
//+kubebuilder:rbac:groups=rbac,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac,resources=clusterroles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=rbac,resources=clusterroles/finalizers,verbs=update
//+kubebuilder:rbac:groups=rbac,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac,resources=clusterrolebindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=rbac,resources=clusterrolebindings/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Namespace object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// Fetch namespace
	var namespace corev1.Namespace

	if err := r.Get(ctx, req.NamespacedName, &namespace); err != nil {
		if apierrors.IsNotFound(err) {
			//Ignore not-found errors
			return ctrl.Result{}, nil
		}
		log.Log.Error(err, "Unable to fetch namespace")
		return ctrl.Result{}, err
	}

	// Detect annotation
	labelIsPresent := len(namespace.Annotations[namespaceCapabilityNameLabel]) > 0

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "providerconfig-" + namespace.Name,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:         []string{"get"},
				APIGroups:     []string{"aws.crossplane.io"},
				Resources:     []string{"providerconfigs"},
				ResourceNames: []string{namespace.Name + "-aws"},
			},
		},
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "providerconfig-" + namespace.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     namespace.Name,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
	}

	if labelIsPresent {

		log.Log.Info("Capability detected on namespace " + namespace.Name)
		controllerutil.SetControllerReference(&namespace, clusterRole, r.Scheme)

		// Create clusterrole if not exists
		if err := r.Create(ctx, clusterRole); err != nil {
			if apierrors.IsAlreadyExists(err) {
				log.Log.Info("ClusterRole " + clusterRole.Name + " already exists for " + namespace.Name)

				// Update clusterrole in case of changes
				// TODO: Check if currently deployed clusterrole matches clusterrole to deploy before applying update
				if err := r.Update(ctx, clusterRole); err != nil {
					log.Log.Info("Unable to update role " + clusterRole.Name + " for " + namespace.Name)
				} else {
					log.Log.Info("ClusterRole " + clusterRole.Name + " for " + namespace.Name + " has been updated")
				}
				err = nil
			}
			if err != nil {
				log.Log.Info("Unable to make ClusterRole " + clusterRole.Name + " for " + namespace.Name)
			}
			return ctrl.Result{}, err
		} else {
			log.Log.Info("ClusterRole " + clusterRole.Name + " created for " + namespace.Name)
		}

		// Create clusterrolebinding if not exists
		if err := r.Create(ctx, clusterRoleBinding); err != nil {
			if apierrors.IsAlreadyExists(err) {
				log.Log.Info("ClusterRoleBinding " + clusterRoleBinding.Name + " already exists for " + namespace.Name)

				// Update clusterrolebinding in case of changes
				// TODO: Check if currently deployed clusterrolebinding matches clusterrolebindnig to deploy before applying update
				if err := r.Update(ctx, clusterRoleBinding); err != nil {
					log.Log.Info("Unable to update clusterrolebinding " + clusterRoleBinding.Name + " for " + namespace.Name)
				} else {
					log.Log.Info("ClusterRoleBinding " + clusterRoleBinding.Name + " for " + namespace.Name + " has been updated")
				}
				err = nil
			}
			if err != nil {
				log.Log.Info("Unable to make ClusterRoleBinding " + clusterRoleBinding.Name + " for " + namespace.Name)
			}
			return ctrl.Result{}, err
		} else {
			log.Log.Info("ClusterRoleBinding " + clusterRoleBinding.Name + " created for " + namespace.Name)
		}
		// Create providerconfig if not exists
		return ctrl.Result{}, nil

	} else {
		log.Log.Info("Capability not detected on namespace " + namespace.Name)

		// Delete clusterrole if exists
		if err := r.Delete(ctx, clusterRole); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				log.Log.Info("ClusterRole does not exist")
				// return ctrl.Result{}, nil
				err = nil
			} else {
				return ctrl.Result{}, err
			}
		} else {
			log.Log.Info("ClusterRole deleted")
		}

		// Delete clusterrolebinding if exists
		if err := r.Delete(ctx, clusterRoleBinding); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				log.Log.Info("ClusterRoleBinding does not exist")
				// return ctrl.Result{}, nil
				err = nil
			} else {
				return ctrl.Result{}, err
			}
		} else {
			log.Log.Info("ClusterRoleBinding deleted")
		}
		// Delete providerconfig if exists

	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Complete(r)
}

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
	"os"
	"strings"

	provideraws "github.com/crossplane/provider-aws/apis/v1beta1"
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
//+kubebuilder:rbac:groups=rbac,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac,resources=roles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=rbac,resources=roles/finalizers,verbs=update
//+kubebuilder:rbac:groups=rbac,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac,resources=rolebindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=rbac,resources=rolebindings/finalizers,verbs=update
//+kubebuilder:rbac:groups=aws.crossplane.io,resources=providerconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=aws.crossplane.io,resources=providerconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=aws.crossplane.io,resources=providerconfigs/finalizers,verbs=update

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

	log.Log.Info("Processing namespace " + namespace.Name)

	// Detect annotation
	getCrossplaneEnabledAnnotationName, err := getAnnotationEnvVarName("DFDS_CROSSPLANE_ENABLED_ANNOTATION_NAME", "dfds-crossplane-enabled")
	if err != nil {
		log.Log.Error(err, "unable to get Crossplane Enabled annotation name")
	}

	getAWSAccountIDAnnotationName, err := getAnnotationEnvVarName("DFDS_CROSSPLANE_AWS_ACCOUNT_ID_ANNOTATION_NAME", "dfds-aws-account-id")
	if err != nil {
		log.Log.Error(err, "unable to get AWS Account annotation name")
	}

	crossplaneEnabled := "false"
	if len(namespace.Annotations[getCrossplaneEnabledAnnotationName]) > 0 {
		log.Log.Info("Crossplane annotation value found: " + namespace.Annotations[getCrossplaneEnabledAnnotationName])
		crossplaneEnabled = namespace.Annotations[getCrossplaneEnabledAnnotationName]
	}

	awsAccountId := ""
	if len(namespace.Annotations[getAWSAccountIDAnnotationName]) > 0 {
		log.Log.Info("AWS Account ID value found: " + namespace.Annotations[getAWSAccountIDAnnotationName])
		awsAccountId = namespace.Annotations[getAWSAccountIDAnnotationName]
	}

	// TODO: Obtain this clusterrole info from a Configmap or other source, rather than hard code
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace.Name + "-aws",
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

	// TODO: Obtain this clusterrolebinding info from a Configmap or other source, rather than hard code
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace.Name + "-aws",
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

	roleArn := "arn:aws:iam::" + awsAccountId + ":role/crossplane-deploy"

	// TODO: Obtain this providerconfig info from a Configmap or other source, rather than hard code
	providerAWS := &provideraws.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace.Name + "-aws",
		},
		Spec: provideraws.ProviderConfigSpec{
			Credentials: provideraws.ProviderCredentials{
				Source: "InjectedIdentity",
			},
			AssumeRoleARN: &roleArn,
		},
	}

	allowedAPIGroups, err := getDFDSCrossplaneAPIGroups("DFDS_CROSSPLANE_PKG_ALLOWED_API_GROUPS", []string{"xplane.dfds.cloud"})
	if err != nil {
		log.Log.Error(err, "unable to get API Groups")
	}

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespace.Name + "-dfds-xplane-cfg-pkg",
			Namespace: namespace.Name,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"*"},
				APIGroups: allowedAPIGroups,
				Resources: []string{"*"},
			},
		},
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespace.Name + "-dfds-xplane-cfg-pkg",
			Namespace: namespace.Name,
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
			Kind:     "Role",
			Name:     role.Name,
		},
	}

	if crossplaneEnabled == "true" {

		log.Log.Info("Detected Crossplane enabled on namespace " + namespace.Name)

		if awsAccountId == "" {
			log.Log.Info("No AWS Account ID found on namespace. Skipping creation of resources")
		} else {
			controllerutil.SetControllerReference(&namespace, clusterRole, r.Scheme)
			controllerutil.SetControllerReference(&namespace, clusterRoleBinding, r.Scheme)
			controllerutil.SetControllerReference(&namespace, providerAWS, r.Scheme)
			controllerutil.SetControllerReference(&namespace, role, r.Scheme)
			controllerutil.SetControllerReference(&namespace, roleBinding, r.Scheme)

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
					return ctrl.Result{}, err
				}
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
					return ctrl.Result{}, err
				}
			} else {
				log.Log.Info("ClusterRoleBinding " + clusterRoleBinding.Name + " created for " + namespace.Name)
			}

			// Create providerconfig if not exists
			if err := r.Create(ctx, providerAWS); err != nil {
				if apierrors.IsAlreadyExists(err) {
					log.Log.Info("ProviderConfig " + providerAWS.Name + " already exists for " + namespace.Name)

					// Update providerconfig in case of changes
					/*
						For some reason we need to retrieve the providerConfig and set the ResourceVersion on our referenced
						object otherwise we get an error saying "ResourceVersion" must be specified for an update. Maybe there
						is a better way to do it, but this seems to work for now.
						TODO: Investigate better way of performing updte on ProviderConfig
					*/

					getProviderConfig := &provideraws.ProviderConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name: "" + namespace.Name + "-aws",
						},
						Spec: provideraws.ProviderConfigSpec{
							Credentials: provideraws.ProviderCredentials{
								Source: "InjectedIdentity",
							},
							AssumeRoleARN: &roleArn,
						},
					}

					if err := r.Get(ctx, client.ObjectKey{
						Namespace: namespace.Name,
						Name:      providerAWS.Name,
					}, getProviderConfig); err != nil {
						log.Log.Info("Unable to get ProviderConfig \n\n" + err.Error())
					} else {
						log.Log.Info("Successfully retrieved ProviderConfig. ResourceVersion: " + getProviderConfig.ResourceVersion)
						providerAWS.SetResourceVersion(getProviderConfig.GetResourceVersion())
					}

					if err := r.Update(ctx, providerAWS); err != nil {
						log.Log.Info("Unable to update providerconfig " + providerAWS.Name + " for " + providerAWS.Name + "\n\n" + err.Error())
					} else {
						log.Log.Info("ProviderConfig " + providerAWS.Name + " for " + namespace.Name + " has been updated")
					}

					err = nil
				}

				//TODO: Check whether the ProviderConfig Kind exists in the cluster and silence error

				if err != nil {
					log.Log.Info("Unable to make ProviderConfig " + providerAWS.Name + " for " + namespace.Name)
					return ctrl.Result{}, err
				}
			} else {
				log.Log.Info("ProviderConfig " + providerAWS.Name + " created for " + namespace.Name)
			}

			// Create role if not exists
			if err := r.Create(ctx, role); err != nil {
				if apierrors.IsAlreadyExists(err) {
					log.Log.Info("role " + role.Name + " already exists for " + namespace.Name)

					// Update role in case of changes
					// TODO: Check if currently deployed role matches role to deploy before applying update
					if err := r.Update(ctx, role); err != nil {
						log.Log.Info("Unable to update role " + role.Name + " for " + namespace.Name)
					} else {
						log.Log.Info("role " + role.Name + " for " + namespace.Name + " has been updated")
					}
					err = nil
				}
				if err != nil {
					log.Log.Info("Unable to make role " + role.Name + " for " + namespace.Name)
					return ctrl.Result{}, err
				}
			} else {
				log.Log.Info("role " + role.Name + " created for " + namespace.Name)
			}

			// Create rolebinding if not exists
			if err := r.Create(ctx, roleBinding); err != nil {
				if apierrors.IsAlreadyExists(err) {
					log.Log.Info("roleBinding " + roleBinding.Name + " already exists for " + namespace.Name)

					// Update roleBinding in case of changes
					// TODO: Check if currently deployed roleBinding matches clusterrolebindnig to deploy before applying update
					if err := r.Update(ctx, roleBinding); err != nil {
						log.Log.Info("Unable to update roleBinding " + roleBinding.Name + " for " + namespace.Name)
					} else {
						log.Log.Info("roleBinding " + roleBinding.Name + " for " + namespace.Name + " has been updated")
					}
					err = nil
				}
				if err != nil {
					log.Log.Info("Unable to make roleBinding " + roleBinding.Name + " for " + namespace.Name)
					return ctrl.Result{}, err
				}
			} else {
				log.Log.Info("roleBinding " + roleBinding.Name + " created for " + namespace.Name)
			}
		}

		return ctrl.Result{}, nil

	} else {
		log.Log.Info("Detected Crossplane not enabled on namespace " + namespace.Name)

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
		if err := r.Delete(ctx, providerAWS); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				log.Log.Info("ProviderConfig does not exist")
				// return ctrl.Result{}, nil
				err = nil
			} else {
				return ctrl.Result{}, err
			}
		} else {
			log.Log.Info("ProviderConfig deleted")
		}

		// Delete role if exists
		if err := r.Delete(ctx, role); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				log.Log.Info("role does not exist")
				// return ctrl.Result{}, nil
				err = nil
			} else {
				return ctrl.Result{}, err
			}
		} else {
			log.Log.Info("role deleted")
		}

		// Delete roleBinding if exists
		if err := r.Delete(ctx, roleBinding); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				log.Log.Info("roleBinding does not exist")
				// return ctrl.Result{}, nil
				err = nil
			} else {
				return ctrl.Result{}, err
			}
		} else {
			log.Log.Info("roleBinding deleted")
		}

	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Complete(r)
}

func getAnnotationEnvVarName(envVarName string, defaultVarName string) (string, error) {
	annotation, found := os.LookupEnv(envVarName)
	if !found {
		ctrl.Log.Info("No " + envVarName + " environment variable set. Using default of " + defaultVarName)
		return defaultVarName, nil
	}
	return annotation, nil
}

func getDFDSCrossplaneAPIGroups(envVarName string, defaultValues []string) ([]string, error) {
	groups, found := os.LookupEnv(envVarName)

	if !found {
		ctrl.Log.Info("No " + envVarName + " environment variable set. Using default value of " + strings.Join(defaultValues, ", "))
		return defaultValues, nil
	}

	s := strings.Split(groups, ",")

	for count, item := range s {
		s[count] = strings.TrimSpace(item)
	}

	return s, nil

}

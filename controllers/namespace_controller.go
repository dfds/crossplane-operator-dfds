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

	//configMap := getConfigMap("DFDS_CROSSPLANE_CONFIGMAP_NAME", "DFDS_CROSSPLANE_CONFIGMAP_NAMESPACE")

	// TODO: Obtain this clusterrole info from a Configmap or other source, rather than hard code
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

	// TODO: Obtain this clusterrolebinding info from a Configmap or other source, rather than hard code
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

	// TODO: We need to obtain this roleArn. Currently namespaces are not annotated with AWS Account ID
	// so we may need to query the capability service to get them which is not ideal. It would be good
	// if we could annotate namespaces on creation
	roleArn := "arn:aws:iam::" + awsAccountId + ":role/crossplane-deploy"

	// TODO: Obtain this providerconfig info from a Configmap or other source, rather than hard code
	providerAWS := &provideraws.ProviderConfig{
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

	if crossplaneEnabled == "true" {

		log.Log.Info("Detected Crossplane enabled on namespace " + namespace.Name)

		if awsAccountId == "" {
			log.Log.Info("No AWS Account ID found on namespace. Skipping creation of resources")
		} else {
			controllerutil.SetControllerReference(&namespace, clusterRole, r.Scheme)
			controllerutil.SetControllerReference(&namespace, clusterRoleBinding, r.Scheme)
			controllerutil.SetControllerReference(&namespace, providerAWS, r.Scheme)

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

// func getConfigMap(configMapNameEnvVar string, configMapNamespaceEnvVar string) corev1.ConfigMap {
// 	configMapNameEnvVar, found := os.LookupEnv(configMapNameEnvVar)
// 	if !found {

// 	}

// 	configMapNamespaceEnvVar, found := os.LookupEnv(configMapNamespaceEnvVar)
// 	if !found {

// 	}
// }

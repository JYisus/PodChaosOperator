/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	chaosv1 "github.com/jyisus/pod-chaos-operator/api/v1"
)

const blacklistFileName = "blacklist.yml"

// PodChaosMonkeyReconciler reconciles a PodChaosMonkey object
type PodChaosMonkeyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=chaos.jyisus.com,resources=podchaosmonkeys,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=chaos.jyisus.com,resources=podchaosmonkeys/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=chaos.jyisus.com,resources=podchaosmonkeys/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PodChaosMonkey object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.2/pkg/reconcile
func (r *PodChaosMonkeyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// TODO(user): your logic here

	logger := log.FromContext(ctx)

	monkey := &chaosv1.PodChaosMonkey{}
	err := r.Client.Get(ctx, req.NamespacedName, monkey)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	var result *reconcile.Result

	blacklistConfigmap := getBlacklistConfigMap(monkey)
	err = controllerutil.SetControllerReference(monkey, blacklistConfigmap, r.Scheme)
	if err != nil {
		logger.Error(err, "failed to set controller reference on blacklist configmap")
		return reconcile.Result{}, err
	}
	result, err = r.ensureConfigmap(ctx, blacklistConfigmap)
	if err != nil {
		logger.Error(err, "failed to ensure configmap creation")
		return *result, err
	}

	configmap := getConfigMap(monkey)
	err = controllerutil.SetControllerReference(monkey, configmap, r.Scheme)
	if err != nil {
		logger.Error(err, "failed to set controller reference on configmap")
		return reconcile.Result{}, err
	}

	result, err = r.ensureConfigmap(ctx, configmap)
	if err != nil {
		logger.Error(err, "failed to ensure configmap creation")
		return *result, err
	}
	deployment := getChaosMonkeyDeployment(monkey, blacklistConfigmap, configmap)
	err = controllerutil.SetControllerReference(monkey, deployment, r.Scheme)
	if err != nil {
		logger.Error(err, "failed to set controller reference on deployment")
		return reconcile.Result{}, err
	}
	result, err = r.ensureDeployment(ctx, monkey, deployment)
	if err != nil {
		logger.Error(err, "failed to ensure deployment")
		return *result, err
	}
	result, err = r.handleConfigmapChanges(ctx, monkey, blacklistConfigmap, configmap, deployment)
	if err != nil {
		logger.Error(err, "failed to handle deployment changes")
		return *result, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodChaosMonkeyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&chaosv1.PodChaosMonkey{}).
		Complete(r)
}

func (r *PodChaosMonkeyReconciler) getPodChaosMonkeyConfigmap(monkey *chaosv1.PodChaosMonkey) *corev1.ConfigMap {
	configmap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-configmap", monkey.Name),
			Namespace: monkey.Namespace,
		},
		Data: map[string]string{
			"SCHEDULE":          monkey.Spec.Schedule,
			"SCHEDULE_FORMAT":   monkey.Spec.ScheduleFormat,
			"NAMESPACE":         monkey.Namespace,
			"IS_INSIDE_CLUSTER": "true",
		},
	}
	err := controllerutil.SetControllerReference(monkey, configmap, r.Scheme)
	if err != nil {
		fmt.Printf("failed to set controller reference: %s", err) // todo add logger
		return nil
	}
	return configmap
}

func (r *PodChaosMonkeyReconciler) ensureConfigmap(
	ctx context.Context,
	configmap *corev1.ConfigMap,
) (*reconcile.Result, error) {
	found := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      configmap.Name,
		Namespace: configmap.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {
		fmt.Println(err)
		fmt.Printf("Creating a new configmap %s on namespace %s\n", configmap.Name, configmap.Namespace)
		err = r.Client.Create(ctx, configmap)
		if err != nil {
			fmt.Printf("failed to create configmap: %s", err)
			return &reconcile.Result{}, err
		}
		return nil, nil
	}
	if err != nil {
		fmt.Println(err)
		return &reconcile.Result{}, err
	}

	return nil, nil
}

func (r *PodChaosMonkeyReconciler) ensureDeployment(ctx context.Context, monkey *chaosv1.PodChaosMonkey, deployment *appsv1.Deployment) (*reconcile.Result, error) {
	found := &appsv1.Deployment{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      deployment.Name,
		Namespace: monkey.Namespace,
	}, found)
	//found.Spec.Template.Spec.Containers[0].
	if err != nil && errors.IsNotFound(err) {
		err = r.Client.Create(ctx, deployment)

		if err != nil {
			// Deployment failed
			return &reconcile.Result{}, err
		} else {
			// Deployment was successful
			return nil, nil
		}
	} else if err != nil {
		// Error that isn't due to the deployment not existing
		return &reconcile.Result{}, err
	}

	return nil, nil
}

func (r *PodChaosMonkeyReconciler) handleConfigmapChanges(
	ctx context.Context,
	monkey *chaosv1.PodChaosMonkey,
	blacklistConfigMap *corev1.ConfigMap,
	configMap *corev1.ConfigMap,
	deployment *appsv1.Deployment,
) (*reconcile.Result, error) {
	foundConfigmap := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      configMap.Name,
		Namespace: monkey.Namespace,
	}, foundConfigmap)
	if err != nil {
		// The deployment may not have been created yet, so requeue
		return &reconcile.Result{RequeueAfter: 5 * time.Second}, err
	}

	var changed bool

	if !reflect.DeepEqual(foundConfigmap.Data, configMap.Data) {
		fmt.Println("Configmap changed")
		err = r.Client.Update(ctx, configMap)
		if err != nil {
			return &reconcile.Result{}, err
		}
	}

	foundBlacklistConfig := &corev1.ConfigMap{}
	err = r.Client.Get(ctx, types.NamespacedName{
		Name:      configMap.Name,
		Namespace: monkey.Namespace,
	}, foundBlacklistConfig)
	if err != nil {
		// The deployment may not have been created yet, so requeue
		return &reconcile.Result{RequeueAfter: 5 * time.Second}, err
	}

	if !reflect.DeepEqual(foundBlacklistConfig.Data, blacklistConfigMap.Data) {
		fmt.Println("Blacklist changed")
		fmt.Printf("%+v", foundBlacklistConfig.Data)
		fmt.Printf("%+v", blacklistConfigMap.Data)
		err = r.Client.Update(ctx, blacklistConfigMap)
		if err != nil {
			return &reconcile.Result{}, err
		}
		changed = true
	}

	if changed {
		// Spec updated - return and requeue
		err = r.Client.Delete(ctx, deployment)
		if err != nil {
			return &reconcile.Result{}, err
		}

		deployment.ResourceVersion = ""

		err = r.Client.Create(ctx, deployment)
		if err != nil {
			return &reconcile.Result{}, err
		}

		return &reconcile.Result{Requeue: true}, nil
	}

	return nil, nil
}

func getConfigMap(monkey *chaosv1.PodChaosMonkey) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-config", monkey.Name),
			Namespace: monkey.Namespace,
		},
		Data: map[string]string{
			"SCHEDULE":          monkey.Spec.Schedule,
			"SCHEDULE_FORMAT":   monkey.Spec.ScheduleFormat,
			"NAMESPACE":         monkey.Spec.TargetNamespace,
			"IS_INSIDE_CLUSTER": "true",
		},
	}
}

func getBlacklistConfigMapName(monkey *chaosv1.PodChaosMonkey) string {
	return fmt.Sprintf("%s-blacklist-config", monkey.Name)
}

func getBlacklistConfigMap(monkey *chaosv1.PodChaosMonkey) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getBlacklistConfigMapName(monkey),
			Namespace: monkey.Namespace,
		},
		Data: map[string]string{
			blacklistFileName: monkey.Spec.Blacklist.AsString(),
		},
	}
}

func getChaosMonkeyDeployment(
	monkey *chaosv1.PodChaosMonkey,
	blacklistConfigmap *corev1.ConfigMap,
	configmap *corev1.ConfigMap,
) *appsv1.Deployment {
	labels := map[string]string{
		"app":               "pod-chaos-monkey",
		"podchaosmonkey_cr": monkey.Name,
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      monkey.Name,
			Namespace: monkey.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &monkey.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{{
						Name: getBlacklistConfigMapName(monkey),
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: getBlacklistConfigMapName(monkey),
								},
								Items: []corev1.KeyToPath{{Key: blacklistFileName, Path: blacklistFileName}},
							},
						},
					}},
					ServiceAccountName: "pod-chaos-monkey-sa",
					Containers: []corev1.Container{{
						Image: monkey.Spec.Image,
						Name:  "pod-chaos-monkey",
						VolumeMounts: []corev1.VolumeMount{{
							Name:      blacklistConfigmap.Name,
							MountPath: fmt.Sprintf("/app/%s", blacklistFileName),
							SubPath:   blacklistFileName,
							ReadOnly:  true,
						}},
						EnvFrom: []corev1.EnvFromSource{{
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: configmap.Name,
								},
							},
						}},
					}},
				},
			},
		},
	}
	return deployment
}

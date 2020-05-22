/*


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
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/yaml"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	scorecardv1alpha1 "github.com/joelanford/scorecard-operator/api/v1alpha1"
)

// TestReconciler reconciles a Test object
type TestReconciler struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	KubernetesClient kubernetes.Interface
}

// +kubebuilder:rbac:groups=scorecard.operatorframework.io,resources=tests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scorecard.operatorframework.io,resources=tests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete

func (r *TestReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("test", req.NamespacedName)

	test := &scorecardv1alpha1.Test{}
	if err := r.Client.Get(context.TODO(), req.NamespacedName, test); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	pod := buildPod(*test)
	podNn := types.NamespacedName{
		Namespace: pod.GetNamespace(),
		Name:      pod.GetName(),
	}
	err := r.Client.Get(context.TODO(), podNn, &pod)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	if apierrors.IsNotFound(err) {
		if err := r.Client.Create(context.TODO(), &pod); err != nil {
			return ctrl.Result{}, fmt.Errorf("error creating test pod: %w", err)
		}
	}

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := r.Client.Get(context.TODO(), req.NamespacedName, test); err != nil {
			return err
		}
		test.Status.Phase = pod.Status.Phase
		if err := r.Client.Status().Update(context.TODO(), test); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error updating pod phase in test status: %w", err)
	}

	switch pod.Status.Phase {
	case v1.PodFailed, v1.PodSucceeded:
		log, err := r.KubernetesClient.CoreV1().Pods(pod.GetNamespace()).GetLogs(pod.GetName(), &v1.PodLogOptions{Container: "test"}).DoRaw(context.TODO())
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting pod logs: %w", err)
		}
		res := &scorecardv1alpha1.Test{}
		if err := yaml.Unmarshal(log, res); err != nil {
			return ctrl.Result{}, fmt.Errorf("error unmarshaling logs as test: %w", err)
		}
		test.Status.Results = res.Status.Results
		if err := r.Client.Status().Update(context.TODO(), test); err != nil {
			return ctrl.Result{}, fmt.Errorf("error updating results in test status: %w", err)
		}
	case v1.PodPending, v1.PodRunning, v1.PodUnknown:
	default:
	}

	return ctrl.Result{}, nil
}

func buildPod(test scorecardv1alpha1.Test) v1.Pod {
	injectBundle := true
	if test.Spec.BundleConfigMap == "" {
		injectBundle = false
	}

	serviceAccount := "default"
	if test.Spec.ServiceAccount != "" {
		serviceAccount = test.Spec.ServiceAccount
	}

	ownerRef := metav1.NewControllerRef(&test, test.GroupVersionKind())
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            test.Name,
			Namespace:       test.Namespace,
			Labels:          test.Spec.Labels,
			OwnerReferences: []metav1.OwnerReference{*ownerRef},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            "test",
					Image:           test.Spec.Image,
					ImagePullPolicy: v1.PullIfNotPresent,
					Command:         test.Spec.Entrypoint,
				},
			},
			RestartPolicy:      v1.RestartPolicyNever,
			ServiceAccountName: serviceAccount,
		},
	}
	if injectBundle {
		defaultBundleMode := int32(0644)
		bundleConfigMapOptional := false
		pod.Spec.Volumes = []v1.Volume{
			{
				Name: "bundle-tgz",
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: test.Spec.BundleConfigMap,
						},
						DefaultMode: &defaultBundleMode,
						Optional:    &bundleConfigMapOptional,
					},
				},
			},
			{
				Name: "bundle",
				VolumeSource: v1.VolumeSource{
					EmptyDir: &v1.EmptyDirVolumeSource{},
				},
			},
		}
		pod.Spec.InitContainers = []v1.Container{
			{
				Name:            "extract-bundle",
				Image:           "busybox",
				ImagePullPolicy: v1.PullIfNotPresent,
				Args:            []string{"tar", "xvzf", "/in/bundle.tar.gz", "-C", "/out"},
				VolumeMounts: []v1.VolumeMount{
					{
						MountPath: "/in",
						Name:      "bundle-tgz",
						ReadOnly:  true,
					},
					{
						MountPath: "/out",
						Name:      "bundle",
						ReadOnly:  false,
					},
				},
			},
		}
		pod.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{
			{
				MountPath: "/bundle",
				Name:      "bundle",
				ReadOnly:  true,
			},
		}
	}
	return pod
}

func (r *TestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	var err error
	r.KubernetesClient, err = kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&scorecardv1alpha1.Test{}).
		Owns(&v1.Pod{}).
		Complete(r)
}

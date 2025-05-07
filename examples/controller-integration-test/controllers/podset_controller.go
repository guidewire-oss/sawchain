package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "example/api/v1"
)

type PodSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *PodSetReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the PodSet resource
	podSet := &v1.PodSet{}
	if err := r.Get(ctx, req.NamespacedName, podSet); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// List pods owned by this PodSet resource
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList,
		client.InNamespace(req.Namespace),
		client.MatchingLabels(map[string]string{"app": podSet.Name}),
	); err != nil {
		return reconcile.Result{}, err
	}

	// Track existing pods
	existingPods := make(map[string]corev1.Pod)
	for _, pod := range podList.Items {
		if pod.DeletionTimestamp.IsZero() {
			existingPods[pod.Name] = pod
		}
	}

	// Create or update desired pods
	replicas := *podSet.Spec.Replicas
	podNames := make([]string, 0, replicas)
	for i := 0; i < replicas; i++ {
		podName := fmt.Sprintf("%s-%d", podSet.Spec.Template.Name, i)
		podNames = append(podNames, podName)

		pod := r.constructPodForPodSet(podName, podSet)

		if _, exists := existingPods[podName]; exists {
			// Update existing pod
			if err := r.Update(ctx, pod); err != nil {
				return reconcile.Result{}, err
			}
			log.Info("Updated pod", "pod", podName)
		} else {
			// Create new pod
			if err := controllerutil.SetControllerReference(podSet, pod, r.Scheme); err != nil {
				return reconcile.Result{}, err
			}
			if err := r.Create(ctx, pod); err != nil {
				return reconcile.Result{}, err
			}
			log.Info("Created pod", "pod", podName)
		}
		// Remove from map to track which are no longer needed
		delete(existingPods, podName)
	}

	// Delete extra pods
	for _, pod := range existingPods {
		if err := r.Delete(ctx, &pod); err != nil {
			return reconcile.Result{}, err
		}
		log.Info("Deleted pod", "pod", pod.Name)
	}

	// Update status with list of pod names
	podSet.Status.Pods = podNames
	if err := r.Status().Update(ctx, podSet); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *PodSetReconciler) constructPodForPodSet(podName string, podSet *v1.PodSet) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: podSet.Namespace,
			Labels: map[string]string{
				"app": podSet.Name,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{},
		},
	}
	for _, containerSpec := range podSet.Spec.Template.Containers {
		container := corev1.Container{
			Name:  containerSpec.Name,
			Image: containerSpec.Image,
		}
		pod.Spec.Containers = append(pod.Spec.Containers, container)
	}
	return pod
}

func (r *PodSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.PodSet{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

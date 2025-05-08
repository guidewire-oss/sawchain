package v1

import (
	"context"
	"fmt"
	"regexp"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var podSetLog = logf.Log.WithName("podset-resource")

func (r *PodSet) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(r).Complete()
}

// +kubebuilder:webhook:mutating=true,path=/mutate-podset,groups=apps.example.com,resources=podsets,versions=v1,verbs=create;update,name=mpodset.kb.io,sideEffects=None,admissionReviewVersions=v1,failurePolicy=fail

// PodSet mutating webhook does the following:
// - defaults replicas to 1
// - adds an annotation

var _ admission.CustomDefaulter = &PodSet{}

// Default implements webhook.CustomDefaulter.
func (r *PodSet) Default(ctx context.Context, obj runtime.Object) error {
	podSetLog.Info("default", "name", r.Name)

	podSet, ok := obj.(*PodSet)
	if !ok {
		return fmt.Errorf("expected a PodSet but got a %T", obj)
	}

	// Default replicas to 1 if not set
	if podSet.Spec.Replicas == nil {
		replicas := 1
		podSet.Spec.Replicas = &replicas
	}

	// Add an annotation
	if podSet.Annotations == nil {
		podSet.Annotations = make(map[string]string)
	}
	podSet.Annotations["apps.example.com/defaulted"] = "true"

	return nil
}

// +kubebuilder:webhook:mutating=false,path=/validate-podset,groups=apps.example.com,resources=podsets,versions=v1,verbs=create;update,name=vpodset.kb.io,sideEffects=None,admissionReviewVersions=v1,failurePolicy=fail

// PodSet validating webhook rejects the following:
// - negative replicas
// - invalid pod names
// - invalid images

var _ admission.CustomValidator = &PodSet{}

// ValidateCreate implements webhook.CustomValidator.
func (r *PodSet) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	podSetLog.Info("validate create", "name", r.Name)

	podSet, ok := obj.(*PodSet)
	if !ok {
		return nil, fmt.Errorf("expected a PodSet but got a %T", obj)
	}

	return nil, r.validatePodSet(podSet)
}

// ValidateUpdate implements webhook.CustomValidator.
func (r *PodSet) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	podSetLog.Info("validate update", "name", r.Name)

	podSet, ok := newObj.(*PodSet)
	if !ok {
		return nil, fmt.Errorf("expected a PodSet but got a %T", newObj)
	}

	return nil, r.validatePodSet(podSet)
}

// ValidateDelete implements webhook.CustomValidator.
func (r *PodSet) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	podSetLog.Info("validate delete", "name", r.Name)
	// No validation on delete
	return nil, nil
}

// validatePodSet validates the PodSet fields.
func (r *PodSet) validatePodSet(podSet *PodSet) error {
	var errs field.ErrorList

	// Validate replicas is not negative
	if podSet.Spec.Replicas != nil && *podSet.Spec.Replicas < 0 {
		errs = append(errs, field.Invalid(
			field.NewPath("spec").Child("replicas"),
			*podSet.Spec.Replicas,
			"replicas count cannot be negative"))
	}

	// Validate pod name
	if podSet.Spec.Template.Name != "" {
		// Check if pod name is valid (DNS-1123 subdomain)
		validPodName := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
		if !validPodName.MatchString(podSet.Spec.Template.Name) {
			errs = append(errs, field.Invalid(
				field.NewPath("spec").Child("template").Child("name"),
				podSet.Spec.Template.Name,
				"pod name must consist of lower case alphanumeric characters or '-', and must start/end with an alphanumeric character"))
		}
	}

	// Validate container images
	for i, container := range podSet.Spec.Template.Containers {
		if container.Image == "" {
			errs = append(errs, field.Required(
				field.NewPath("spec").Child("template").Child("containers").Index(i).Child("image"),
				"image is required"))
		} else {
			// Check if image is valid (repo/image:tag)
			validImage := regexp.MustCompile(`^[a-zA-Z0-9][-_.a-zA-Z0-9]*(/[-_.a-zA-Z0-9]+)*(:[-_.a-zA-Z0-9]+)?$`)
			if !validImage.MatchString(container.Image) {
				errs = append(errs, field.Invalid(
					field.NewPath("spec").Child("template").Child("containers").Index(i).Child("image"),
					container.Image,
					"invalid image format"))
			}
		}
	}

	if len(errs) > 0 {
		groupKind := schema.GroupKind{Group: "apps.example.com", Kind: "PodSet"}
		return apierrors.NewInvalid(groupKind, podSet.Name, errs)
	}

	return nil
}

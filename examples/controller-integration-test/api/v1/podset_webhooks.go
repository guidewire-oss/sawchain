package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// PodSet mutating webhook defaults replicas to 1 and adds an annotation.
var mutatingWebhookHandler = &webhook.Admission{
	Handler: admission.HandlerFunc(func(ctx context.Context, req admission.Request) admission.Response {
		podSet := &PodSet{}

		err := json.Unmarshal(req.Object.Raw, podSet)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
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

		marshaledPodSet, err := json.Marshal(podSet)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPodSet)
	}),
}

// PodSet validating webhook rejects negative replicas, invalid pod names, and invalid images.
var validatingWebhookHandler = &webhook.Admission{
	Handler: admission.HandlerFunc(func(ctx context.Context, req admission.Request) admission.Response {
		podSet := &PodSet{}

		err := json.Unmarshal(req.Object.Raw, podSet)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		var errs field.ErrorList

		// Validate replicas is not negative
		if podSet.Spec.Replicas != nil && *podSet.Spec.Replicas < 0 {
			errs = append(errs, field.Invalid(
				field.NewPath("spec").Child("replicas"),
				*podSet.Spec.Replicas,
				"replicas cannot be negative"))
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
			return admission.Denied(apierrors.NewInvalid(groupKind, podSet.Name, errs).Error())
		}

		return admission.Allowed("")
	}),
}

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Container struct {
	Name  string `json:"name,omitempty"`
	Image string `json:"image,omitempty"`
}

type Template struct {
	Name       string      `json:"name,omitempty"`
	Containers []Container `json:"containers,omitempty"`
}

type PodSetSpec struct {
	Replicas *int     `json:"replicas,omitempty"`
	Template Template `json:"template,omitempty"`
}

type PodSetStatus struct {
	Pods []string `json:"pods,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

type PodSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PodSetSpec   `json:"spec,omitempty"`
	Status PodSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type PodSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PodSet{}, &PodSetList{})
}

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InstanceStackSpec defines the desired state of InstanceStack
type InstanceStackSpec struct {
	FlavorName  string `json:"flavorName,omitempty"`
	ImageName   string `json:"imageName,omitempty"`
	NetworkUUID string `json:"networkUUID,omitempty"`
}

// InstanceStackStatus defines the observed state of InstanceStack
type InstanceStackStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// InstanceStack is the Schema for the instancestacks API
type InstanceStack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstanceStackSpec   `json:"spec,omitempty"`
	Status InstanceStackStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InstanceStackList contains a list of InstanceStack
type InstanceStackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InstanceStack `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InstanceStack{}, &InstanceStackList{})
}

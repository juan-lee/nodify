package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NodeConditionHandlerSpec defines the desired state of NodeConditionHandler
type NodeConditionHandlerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of NodeConditionHandler. Edit nodeconditionhandler_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// NodeConditionHandlerStatus defines the observed state of NodeConditionHandler
type NodeConditionHandlerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// NodeConditionHandler is the Schema for the nodeconditionhandlers API
type NodeConditionHandler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeConditionHandlerSpec   `json:"spec,omitempty"`
	Status NodeConditionHandlerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NodeConditionHandlerList contains a list of NodeConditionHandler
type NodeConditionHandlerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeConditionHandler `json:"items"`
}

func init() { // nolint: gochecknoinits
	SchemeBuilder.Register(&NodeConditionHandler{}, &NodeConditionHandlerList{})
}

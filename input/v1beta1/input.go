// Package v1beta1 contains the input type for this Function
// +kubebuilder:object:generate=true
// +groupName=template.fn.crossplane.io
// +versionName=v1beta1
package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This isn't a custom resource, in the sense that we never install its CRD.
// It is a KRM-like object, so we generate a CRD to describe its schema.

// TODO: Add your input type here! It doesn't need to be called 'PatchWithCondition', you can
// rename it to anything you like.

type Object struct {
	Name                 string `json:"name"`
	SourceFieldPath      string `json:"sourceFieldPath"`
	DestinationFieldPath string `json:"destinationFieldPath"`
	SourceFieldValue     string `json:"sourceFieldValue"`
	FiledValue           string `json:"filedValue"`
	Condition            string `json:"condition"`
}

type Config struct {
	Objs []Object `json:"objects"`
}

// PatchWithCondition can be used to provide input to this Function.
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:categories=crossplane
type PatchWithCondition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Example is an example field. Replace it with whatever input you need. :)
	Cfg Config `json:"config"`
}

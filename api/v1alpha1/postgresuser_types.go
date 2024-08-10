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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PasswordSpec defines the style of the generated password
type PasswordSpec struct {
	Length            int    `json:"length"`
	Numbers           bool   `json:"numbers"`
	NumbersOverride   string `json:"numbersOverride,omitempty"`
	Symbols           bool   `json:"symbols"`
	SymbolsOverride   string `json:"symbolsOverride,omitempty"`
	Alphabets         bool   `json:"alphabets"`
	AlphabetsOverride string `json:"alphabetsOverride,omitempty"`
}

// SecretTemplate defines the structure of the secret to store user credentials
type SecretTemplate struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	TemplateData string            `json:"templateData,omitempty"`
}

// PostgresUserSpec defines the desired state of PostgresUser
type PostgresUserSpec struct {
	// The name of the user to be created
	Username string `json:"username"`
	// The password for the user, if empty a password will be generated
	Password string `json:"password,omitempty"`
	// Specification for generating a password if Password is empty
	PasswordSpec *PasswordSpec `json:"passwordSpec,omitempty"`
	// The reference to the PostgresDatabase resource this user should be associated with
	DatabaseRef corev1.LocalObjectReference `json:"databaseRef"`
	// The name of the secret where the user credentials will be stored
	SecretName string `json:"secretName,omitempty"`
	// Template for configuring the secret
	SecretTemplate *SecretTemplate `json:"secretTemplate,omitempty"`
	// Whether to drop the user from the database when the PostgresUser resource is deleted
	DropOnDelete bool   `json:"dropOnDelete,omitempty"`
	RoleType     string `json:"roleType"` // New field to specify the role type: "owner", "read", or "write"
}

// PostgresUserStatus defines the observed state of PostgresUser
type PostgresUserStatus struct {
	// Indicates whether the user has been successfully created
	Created bool `json:"created"`
	// Any additional message regarding the user's status
	Message         string `json:"message,omitempty"`
	SecretName      string `json:"secretName,omitempty"`
	SecretNamespace string `json:"secretNamespace,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PostgresUser is the Schema for the postgresusers API
type PostgresUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresUserSpec   `json:"spec,omitempty"`
	Status PostgresUserStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PostgresUserList contains a list of PostgresUser
type PostgresUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgresUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostgresUser{}, &PostgresUserList{})
}

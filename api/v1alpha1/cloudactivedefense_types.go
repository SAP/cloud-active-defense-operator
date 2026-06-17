/*
Copyright 2026.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CloudActiveDefenseSpec defines the desired state of CloudActiveDefense.
type CloudActiveDefenseSpec struct {
	// domain is the base domain for external access (e.g. "c-28e44bf.kyma.ondemand.com").
	// If not specified, the operator will attempt to auto-detect it from the Kyma cluster configuration.
	// +optional
	Domain string `json:"domain,omitempty"`

	// controlpanelAPI configures the controlpanel API component
	// +optional
	ControlpanelAPI ControlpanelAPISpec `json:"controlpanelAPI,omitempty"`

	// controlpanelFront configures the controlpanel frontend component
	// +optional
	ControlpanelFront ControlpanelFrontSpec `json:"controlpanelFront,omitempty"`

	// deploymentManager configures the deployment manager component
	// +optional
	DeploymentManager DeploymentManagerSpec `json:"deploymentManager,omitempty"`

	// keycloak configures the keycloak authentication component
	// +optional
	Keycloak KeycloakSpec `json:"keycloak,omitempty"`

	// database configures the shared controlpanel database settings
	// +optional
	Database DatabaseSpec `json:"database,omitempty"`
}

// DatabaseSpec configures the controlpanel PostgreSQL database.
type DatabaseSpec struct {
	// user is the database username; if empty, a random username is generated
	// +optional
	User string `json:"user,omitempty"`

	// password is the database password; if empty, a random password is generated
	// +optional
	Password string `json:"password,omitempty"`

	// port is the database port
	// +kubebuilder:default=5432
	// +optional
	Port int32 `json:"port,omitempty"`
}

// ControlpanelAPISpec configures the controlpanel API.
type ControlpanelAPISpec struct {
	// image is the container image for the controlpanel API
	// +kubebuilder:default="ghcr.io/sap/controlpanel-api:latest"
	// +optional
	Image string `json:"image,omitempty"`

	// deploymentManagerDBPassword is the password for the deployment manager database user;
	// if empty, a random password is generated
	// +optional
	DeploymentManagerDBPassword string `json:"deploymentManagerDBPassword,omitempty"`
}

// ControlpanelFrontSpec configures the controlpanel frontend.
type ControlpanelFrontSpec struct {
	// image is the container image for the controlpanel frontend
	// +kubebuilder:default="ghcr.io/sap/controlpanel-frontend:latest"
	// +optional
	Image string `json:"image,omitempty"`
}

// DeploymentManagerSpec configures the deployment manager.
type DeploymentManagerSpec struct {
	// image is the container image for the deployment manager
	// +kubebuilder:default="ghcr.io/sap/deployment-manager:latest"
	// +optional
	Image string `json:"image,omitempty"`
}

// KeycloakSpec configures the Keycloak authentication component.
type KeycloakSpec struct {
	// image is the container image for Keycloak
	// +kubebuilder:default="ghcr.io/sap/keycloak-controlpanel:latest"
	// +optional
	Image string `json:"image,omitempty"`

	// adminUsername is the Keycloak bootstrap admin username; if empty, a random username is generated
	// +optional
	AdminUsername string `json:"adminUsername,omitempty"`

	// adminPassword is the Keycloak bootstrap admin password; if empty, a random password is generated
	// +optional
	AdminPassword string `json:"adminPassword,omitempty"`

	// database configures keycloak's dedicated PostgreSQL database
	// +optional
	Database KeycloakDatabaseSpec `json:"database,omitempty"`
}

// KeycloakDatabaseSpec configures Keycloak's PostgreSQL database.
type KeycloakDatabaseSpec struct {
	// user is the database username; if empty, a random username is generated
	// +optional
	User string `json:"user,omitempty"`

	// password is the database password; if empty, a random password is generated
	// +optional
	Password string `json:"password,omitempty"`

	// port is the database port
	// +kubebuilder:default=5432
	// +optional
	Port int32 `json:"port,omitempty"`
}

// CloudActiveDefenseStatus defines the observed state of CloudActiveDefense.
type CloudActiveDefenseStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// resolvedDomain is the actual domain being used by the operator.
	// This will be either the user-provided domain or the auto-detected cluster domain.
	// +optional
	ResolvedDomain string `json:"resolvedDomain,omitempty"`

	// conditions represent the current state of the CloudActiveDefense resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// CloudActiveDefense is the Schema for the cloudactivedefenses API
type CloudActiveDefense struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of CloudActiveDefense
	// +required
	Spec CloudActiveDefenseSpec `json:"spec"`

	// status defines the observed state of CloudActiveDefense
	// +optional
	Status CloudActiveDefenseStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// CloudActiveDefenseList contains a list of CloudActiveDefense
type CloudActiveDefenseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []CloudActiveDefense `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CloudActiveDefense{}, &CloudActiveDefenseList{})
}

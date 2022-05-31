/*
Copyright 2022.

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

package v1alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	// ClusterFinalizer allows ContainerdClusterReconciler to clean up resources associated with ContainerdCluster before
	// removing it from the apiserver.
	ClusterFinalizer = "containerdcluster.infrastructure.cluster.x-k8s.io"
)

// ContainerdClusterSpec defines the desired state of ContainerdCluster
type ContainerdClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint APIEndpoint `json:"controlPlaneEndpoint"`

	// FailureDomains are not usulaly defined on the spec.
	// The containerd provider is special since failure domains don't mean anything in a local environment.
	// Instead, the docker cluster controller will simply copy these into the Status and allow the Cluster API
	// controllers to do what they will with the defined failure domains.
	// +optional
	FailureDomains clusterv1alpha3.FailureDomains `json:"failureDomains,omitempty"`

	// LoadBalancer allows defining configurations for the cluster load balancer.
	// +optional
	LoadBalancer ContainerdLoadBalancer `json:"loadBalancer,omitempty"`
}

// ContainerdLoadBalancer allows defining configurations for the cluster load balancer.
type ContainerdLoadBalancer struct {
	// ImageMeta allows customizing the image used for the cluster load balancer.
	ImageMeta `json:",inline"`
}

// ImageMeta allows customizing the image used for components that are not
// originated from the Kubernetes/Kubernetes release process.
type ImageMeta struct {
	// ImageRepository sets the container registry to pull the haproxy image from.
	// if not set, "kindest" will be used instead.
	// +optional
	ImageRepository string `json:"imageRepository,omitempty"`

	// ImageTag allows to specify a tag for the haproxy image.
	// if not set, "v20210715-a6da3463" will be used instead.
	// +optional
	ImageTag string `json:"imageTag,omitempty"`
}

// ContainerdClusterStatus defines the observed state of ContainerdCluster
type ContainerdClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// Ready denotes that the docker cluster (infrastructure) is ready.
	Ready bool `json:"ready"`

	// FailureDomains don't mean much in CAPC since it's all local, but we can see how the rest of cluster API
	// will use this if we populate it.
	FailureDomains clusterv1alpha3.FailureDomains `json:"failureDomains,omitempty"`

	// Conditions defines current service state of the ContainerdCluster.
	// +optional
	Conditions clusterv1alpha3.Conditions `json:"conditions,omitempty"`
}

// APIEndpoint represents a reachable Kubernetes API endpoint.
type APIEndpoint struct {
	// Host is the hostname on which the API server is serving.
	Host string `json:"host"`

	// Port is the port on which the API server is serving.
	Port int `json:"port"`
}

// GetConditions returns the set of conditions for this object.
func (c *ContainerdCluster) GetConditions() clusterv1alpha3.Conditions {
	return c.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (c *ContainerdCluster) SetConditions(conditions clusterv1alpha3.Conditions) {
	c.Status.Conditions = conditions
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ContainerdCluster is the Schema for the containerdclusters API
type ContainerdCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ContainerdClusterSpec   `json:"spec,omitempty"`
	Status ContainerdClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ContainerdClusterList contains a list of ContainerdCluster
type ContainerdClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ContainerdCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ContainerdCluster{}, &ContainerdClusterList{})
}

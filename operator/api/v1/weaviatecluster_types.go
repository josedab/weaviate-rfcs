package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WeaviateCluster is the Schema for the weaviateclusters API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=wvc
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.version`
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.spec.replicas`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type WeaviateCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WeaviateClusterSpec   `json:"spec,omitempty"`
	Status WeaviateClusterStatus `json:"status,omitempty"`
}

// WeaviateClusterSpec defines the desired state of WeaviateCluster
type WeaviateClusterSpec struct {
	// Version specifies the Weaviate version to deploy
	Version string `json:"version"`

	// Replicas is the number of desired pods
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas"`

	// Resources defines the compute resources for Weaviate pods
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Storage defines the storage configuration
	Storage StorageSpec `json:"storage"`

	// AutoScaling enables and configures automatic horizontal scaling
	// +optional
	AutoScaling *AutoScalingSpec `json:"autoScaling,omitempty"`

	// Modules specifies which Weaviate modules to enable
	// +optional
	Modules []ModuleSpec `json:"modules,omitempty"`

	// CloudProvider specifies cloud provider-specific optimizations
	// +optional
	CloudProvider *CloudProviderSpec `json:"cloudProvider,omitempty"`

	// ServiceMesh configures service mesh integration
	// +optional
	ServiceMesh *ServiceMeshSpec `json:"serviceMesh,omitempty"`

	// Image specifies custom container image settings
	// +optional
	Image *ImageSpec `json:"image,omitempty"`

	// Env specifies additional environment variables
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
}

// StorageSpec defines storage configuration
type StorageSpec struct {
	// Size is the requested storage size
	Size string `json:"size"`

	// StorageClassName is the name of the storage class to use
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`

	// VolumeClaimTemplate allows custom PVC configuration
	// +optional
	VolumeClaimTemplate *corev1.PersistentVolumeClaimSpec `json:"volumeClaimTemplate,omitempty"`
}

// AutoScalingSpec defines auto-scaling configuration
type AutoScalingSpec struct {
	// Enabled determines if auto-scaling is active
	Enabled bool `json:"enabled"`

	// MinReplicas is the minimum number of replicas
	// +kubebuilder:validation:Minimum=1
	MinReplicas int32 `json:"minReplicas"`

	// MaxReplicas is the maximum number of replicas
	// +kubebuilder:validation:Minimum=1
	MaxReplicas int32 `json:"maxReplicas"`

	// TargetCPU is the target CPU utilization percentage
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +optional
	TargetCPU int32 `json:"targetCPU,omitempty"`

	// TargetMemory is the target memory utilization percentage
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +optional
	TargetMemory int32 `json:"targetMemory,omitempty"`

	// TargetQPS is the target queries per second per replica
	// +optional
	TargetQPS *int32 `json:"targetQPS,omitempty"`

	// TargetLatency is the target response latency (e.g., "50ms")
	// +optional
	TargetLatency *string `json:"targetLatency,omitempty"`
}

// ModuleSpec defines a Weaviate module configuration
type ModuleSpec struct {
	// Name of the module
	Name string `json:"name"`

	// Enabled determines if the module is active
	Enabled bool `json:"enabled"`

	// Config contains module-specific configuration
	// +optional
	Config map[string]string `json:"config,omitempty"`

	// Resources for module-specific containers
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// CloudProviderSpec defines cloud provider-specific configurations
type CloudProviderSpec struct {
	// Type specifies the cloud provider (aws, gcp, azure)
	Type string `json:"type"`

	// Region specifies the cloud region
	Region string `json:"region"`

	// AWS-specific configuration
	// +optional
	AWS *AWSSpec `json:"aws,omitempty"`

	// GCP-specific configuration
	// +optional
	GCP *GCPSpec `json:"gcp,omitempty"`

	// Azure-specific configuration
	// +optional
	Azure *AzureSpec `json:"azure,omitempty"`
}

// AWSSpec defines AWS-specific optimizations
type AWSSpec struct {
	// EBSOptimized enables EBS optimization
	EBSOptimized bool `json:"ebsOptimized,omitempty"`

	// InstanceType specifies the EC2 instance type
	// +optional
	InstanceType string `json:"instanceType,omitempty"`

	// UseLocalNVMe enables local NVMe storage for cache
	UseLocalNVMe bool `json:"useLocalNVMe,omitempty"`

	// Backup configuration for S3
	// +optional
	Backup *AWSBackupSpec `json:"backup,omitempty"`
}

// AWSBackupSpec defines S3 backup configuration
type AWSBackupSpec struct {
	// Bucket name for backups
	Bucket string `json:"bucket"`

	// Region for the S3 bucket
	// +optional
	Region string `json:"region,omitempty"`
}

// GCPSpec defines GCP-specific optimizations
type GCPSpec struct {
	// UseLocalSSD enables local SSD for cache
	UseLocalSSD bool `json:"useLocalSSD,omitempty"`

	// MachineType specifies the GCE machine type
	// +optional
	MachineType string `json:"machineType,omitempty"`

	// Backup configuration for GCS
	// +optional
	Backup *GCPBackupSpec `json:"backup,omitempty"`
}

// GCPBackupSpec defines GCS backup configuration
type GCPBackupSpec struct {
	// Bucket name for backups
	Bucket string `json:"bucket"`
}

// AzureSpec defines Azure-specific optimizations
type AzureSpec struct {
	// VMSize specifies the Azure VM size
	// +optional
	VMSize string `json:"vmSize,omitempty"`

	// UsePremiumStorage enables premium storage
	UsePremiumStorage bool `json:"usePremiumStorage,omitempty"`

	// Backup configuration for Azure Storage
	// +optional
	Backup *AzureBackupSpec `json:"backup,omitempty"`
}

// AzureBackupSpec defines Azure Storage backup configuration
type AzureBackupSpec struct {
	// StorageAccount name for backups
	StorageAccount string `json:"storageAccount"`

	// Container name for backups
	Container string `json:"container"`
}

// ServiceMeshSpec defines service mesh integration
type ServiceMeshSpec struct {
	// Enabled determines if service mesh integration is active
	Enabled bool `json:"enabled"`

	// Type specifies the service mesh type (istio, linkerd, etc.)
	Type string `json:"type"`

	// MTLS enables mutual TLS
	MTLS bool `json:"mTLS,omitempty"`
}

// ImageSpec defines custom container image settings
type ImageSpec struct {
	// Repository is the container image repository
	Repository string `json:"repository,omitempty"`

	// Tag is the container image tag
	Tag string `json:"tag,omitempty"`

	// PullPolicy is the image pull policy
	// +optional
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`

	// PullSecrets for private registries
	// +optional
	PullSecrets []corev1.LocalObjectReference `json:"pullSecrets,omitempty"`
}

// WeaviateClusterStatus defines the observed state of WeaviateCluster
type WeaviateClusterStatus struct {
	// Phase represents the current phase of the cluster
	Phase string `json:"phase,omitempty"`

	// Replicas is the current number of replicas
	Replicas int32 `json:"replicas,omitempty"`

	// ReadyReplicas is the number of ready replicas
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed spec
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true

// WeaviateClusterList contains a list of WeaviateCluster
type WeaviateClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WeaviateCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WeaviateCluster{}, &WeaviateClusterList{})
}

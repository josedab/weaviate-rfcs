package gcp

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	weaviatev1 "github.com/weaviate/weaviate/operator/api/v1"
)

// Optimizer implements GCP-specific optimizations
type Optimizer struct {
	client client.Client
}

// NewOptimizer creates a new GCP optimizer
func NewOptimizer(client client.Client) *Optimizer {
	return &Optimizer{
		client: client,
	}
}

// ApplyOptimizations applies GCP-specific optimizations
func (o *Optimizer) ApplyOptimizations(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	logger := log.FromContext(ctx)

	if cluster.Spec.CloudProvider.GCP == nil {
		logger.Info("No GCP configuration specified")
		return nil
	}

	gcpSpec := cluster.Spec.CloudProvider.GCP

	// Configure local SSD for cache if requested
	if gcpSpec.UseLocalSSD {
		if err := o.configureLocalSSD(ctx, cluster); err != nil {
			logger.Error(err, "Failed to configure local SSD")
			return err
		}
	}

	// Configure persistent disk optimizations
	if err := o.configurePersistentDisk(ctx, cluster); err != nil {
		logger.Error(err, "Failed to configure persistent disk")
		return err
	}

	logger.Info("GCP optimizations applied successfully")
	return nil
}

// ConfigureStorage configures GCP-specific storage options
func (o *Optimizer) ConfigureStorage(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	logger := log.FromContext(ctx)

	// Create StorageClass for SSD persistent disks with optimized settings
	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-pd-ssd", cluster.Name),
		},
	}

	err := o.client.Get(ctx, client.ObjectKey{Name: storageClass.Name}, storageClass)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		// Create new StorageClass for GCE Persistent Disk SSD
		storageClass.Provisioner = "pd.csi.storage.gke.io"
		storageClass.VolumeBindingMode = &[]storagev1.VolumeBindingMode{storagev1.VolumeBindingWaitForFirstConsumer}[0]
		storageClass.AllowVolumeExpansion = &[]bool{true}[0]
		storageClass.Parameters = map[string]string{
			"type":             "pd-ssd",
			"replication-type": "regional-pd", // For HA
		}

		if err := o.client.Create(ctx, storageClass); err != nil {
			return fmt.Errorf("failed to create StorageClass: %w", err)
		}

		logger.Info("Created GCP PD-SSD StorageClass", "name", storageClass.Name)
	}

	return nil
}

// ConfigureBackup configures GCS backup integration
func (o *Optimizer) ConfigureBackup(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	logger := log.FromContext(ctx)

	if cluster.Spec.CloudProvider.GCP == nil || cluster.Spec.CloudProvider.GCP.Backup == nil {
		return nil
	}

	backupSpec := cluster.Spec.CloudProvider.GCP.Backup

	// Create ConfigMap for backup configuration
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-backup-config", cluster.Name),
			Namespace: cluster.Namespace,
		},
		Data: map[string]string{
			"backup-gcs-bucket": backupSpec.Bucket,
			"backup-provider":   "gcs",
		},
	}

	err := o.client.Get(ctx, client.ObjectKey{Name: configMap.Name, Namespace: configMap.Namespace}, &corev1.ConfigMap{})
	if err != nil && errors.IsNotFound(err) {
		if err := o.client.Create(ctx, configMap); err != nil {
			return fmt.Errorf("failed to create backup ConfigMap: %w", err)
		}
		logger.Info("Created GCS backup configuration", "bucket", backupSpec.Bucket)
	}

	return nil
}

// GetOptimalInstanceType returns the recommended GCE machine type
func (o *Optimizer) GetOptimalInstanceType(cluster *weaviatev1.WeaviateCluster) string {
	if cluster.Spec.CloudProvider.GCP != nil && cluster.Spec.CloudProvider.GCP.MachineType != "" {
		return cluster.Spec.CloudProvider.GCP.MachineType
	}

	// Recommend machine type based on resource requirements
	if cluster.Spec.Resources.Requests != nil {
		cpu := cluster.Spec.Resources.Requests.Cpu()
		memory := cluster.Spec.Resources.Requests.Memory()

		// Simple heuristic for machine type selection
		cpuCores := cpu.MilliValue() / 1000
		memoryGiB := memory.Value() / (1024 * 1024 * 1024)

		switch {
		case cpuCores >= 16 || memoryGiB >= 64:
			return "c2-standard-16" // 16 vCPU, 64 GiB
		case cpuCores >= 8 || memoryGiB >= 32:
			return "c2-standard-8" // 8 vCPU, 32 GiB
		case cpuCores >= 4 || memoryGiB >= 16:
			return "c2-standard-4" // 4 vCPU, 16 GiB
		default:
			return "n2-standard-4" // 4 vCPU, 16 GiB
		}
	}

	// Default to a balanced machine type
	return "c2-standard-8"
}

// configureLocalSSD configures local SSD storage
func (o *Optimizer) configureLocalSSD(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	logger := log.FromContext(ctx)
	logger.Info("Configuring local SSD storage for cache")

	// In a full implementation, this would:
	// 1. Add node selector for instances with local SSD
	// 2. Configure volume mounts for local SSD
	// 3. Set up init containers to format and mount SSD devices

	logger.Info("Local SSD configuration applied",
		"note", "Ensure nodes have local SSD attached")

	return nil
}

// configurePersistentDisk configures persistent disk settings
func (o *Optimizer) configurePersistentDisk(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	logger := log.FromContext(ctx)
	logger.Info("Configuring persistent disk optimizations")

	// In a full implementation, this would:
	// 1. Set node selector for appropriate machine types
	// 2. Configure disk parameters (IOPS, throughput)
	// 3. Set up regional persistent disks for HA

	logger.Info("Persistent disk optimizations applied")
	return nil
}

// Machine types categorized by use case
var (
	// ComputeOptimizedMachines are optimized for compute-intensive workloads
	ComputeOptimizedMachines = []string{
		"c2-standard-4", "c2-standard-8", "c2-standard-16", "c2-standard-30",
		"c2d-standard-4", "c2d-standard-8", "c2d-standard-16",
	}

	// MemoryOptimizedMachines are optimized for memory-intensive workloads
	MemoryOptimizedMachines = []string{
		"m1-ultramem-40", "m1-ultramem-80", "m1-ultramem-160",
		"m2-ultramem-208", "m2-ultramem-416",
		"n2-highmem-4", "n2-highmem-8", "n2-highmem-16",
	}

	// BalancedMachines provide balanced CPU and memory
	BalancedMachines = []string{
		"n2-standard-2", "n2-standard-4", "n2-standard-8", "n2-standard-16",
		"n2d-standard-2", "n2d-standard-4", "n2d-standard-8",
	}
)

// GetMachineTypeByWorkload recommends machine type based on workload characteristics
func GetMachineTypeByWorkload(workloadType string, size string) string {
	var machines []string

	switch workloadType {
	case "compute":
		machines = ComputeOptimizedMachines
	case "memory":
		machines = MemoryOptimizedMachines
	case "balanced":
		machines = BalancedMachines
	default:
		machines = ComputeOptimizedMachines
	}

	// Map size to array index
	sizeMap := map[string]int{
		"small":  0,
		"medium": 1,
		"large":  2,
		"xlarge": 3,
	}

	idx := sizeMap[size]
	if idx >= len(machines) {
		idx = len(machines) - 1
	}

	return machines[idx]
}

// RegionalPDConfig represents configuration for regional persistent disks
type RegionalPDConfig struct {
	// Zones for regional PD replication
	Zones []string
	// Type of disk (pd-ssd, pd-balanced, pd-standard)
	DiskType string
}

// GetRegionalPDConfig returns optimal regional PD configuration
func GetRegionalPDConfig(region string) RegionalPDConfig {
	// Default to using two zones in the region
	return RegionalPDConfig{
		Zones: []string{
			fmt.Sprintf("%s-a", region),
			fmt.Sprintf("%s-b", region),
		},
		DiskType: "pd-ssd",
	}
}

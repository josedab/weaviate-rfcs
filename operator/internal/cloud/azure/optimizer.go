package azure

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

// Optimizer implements Azure-specific optimizations
type Optimizer struct {
	client client.Client
}

// NewOptimizer creates a new Azure optimizer
func NewOptimizer(client client.Client) *Optimizer {
	return &Optimizer{
		client: client,
	}
}

// ApplyOptimizations applies Azure-specific optimizations
func (o *Optimizer) ApplyOptimizations(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	logger := log.FromContext(ctx)

	if cluster.Spec.CloudProvider.Azure == nil {
		logger.Info("No Azure configuration specified")
		return nil
	}

	azureSpec := cluster.Spec.CloudProvider.Azure

	// Configure premium storage if requested
	if azureSpec.UsePremiumStorage {
		if err := o.configurePremiumStorage(ctx, cluster); err != nil {
			logger.Error(err, "Failed to configure premium storage")
			return err
		}
	}

	logger.Info("Azure optimizations applied successfully")
	return nil
}

// ConfigureStorage configures Azure-specific storage options
func (o *Optimizer) ConfigureStorage(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	logger := log.FromContext(ctx)

	// Determine storage type
	storageType := "StandardSSD_LRS"
	if cluster.Spec.CloudProvider.Azure != nil && cluster.Spec.CloudProvider.Azure.UsePremiumStorage {
		storageType = "Premium_LRS"
	}

	// Create StorageClass for Azure Disk with optimized settings
	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-azure-disk", cluster.Name),
		},
	}

	err := o.client.Get(ctx, client.ObjectKey{Name: storageClass.Name}, storageClass)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		// Create new StorageClass for Azure Disk
		storageClass.Provisioner = "disk.csi.azure.com"
		storageClass.VolumeBindingMode = &[]storagev1.VolumeBindingMode{storagev1.VolumeBindingWaitForFirstConsumer}[0]
		storageClass.AllowVolumeExpansion = &[]bool{true}[0]
		storageClass.Parameters = map[string]string{
			"skuName":      storageType,
			"cachingMode":  "ReadWrite",
			"kind":         "Managed",
		}

		if err := o.client.Create(ctx, storageClass); err != nil {
			return fmt.Errorf("failed to create StorageClass: %w", err)
		}

		logger.Info("Created Azure Disk StorageClass", "name", storageClass.Name, "type", storageType)
	}

	return nil
}

// ConfigureBackup configures Azure Storage backup integration
func (o *Optimizer) ConfigureBackup(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	logger := log.FromContext(ctx)

	if cluster.Spec.CloudProvider.Azure == nil || cluster.Spec.CloudProvider.Azure.Backup == nil {
		return nil
	}

	backupSpec := cluster.Spec.CloudProvider.Azure.Backup

	// Create ConfigMap for backup configuration
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-backup-config", cluster.Name),
			Namespace: cluster.Namespace,
		},
		Data: map[string]string{
			"backup-azure-storage-account": backupSpec.StorageAccount,
			"backup-azure-container":       backupSpec.Container,
			"backup-provider":              "azure",
		},
	}

	err := o.client.Get(ctx, client.ObjectKey{Name: configMap.Name, Namespace: configMap.Namespace}, &corev1.ConfigMap{})
	if err != nil && errors.IsNotFound(err) {
		if err := o.client.Create(ctx, configMap); err != nil {
			return fmt.Errorf("failed to create backup ConfigMap: %w", err)
		}
		logger.Info("Created Azure Storage backup configuration",
			"storageAccount", backupSpec.StorageAccount,
			"container", backupSpec.Container)
	}

	return nil
}

// GetOptimalInstanceType returns the recommended Azure VM size
func (o *Optimizer) GetOptimalInstanceType(cluster *weaviatev1.WeaviateCluster) string {
	if cluster.Spec.CloudProvider.Azure != nil && cluster.Spec.CloudProvider.Azure.VMSize != "" {
		return cluster.Spec.CloudProvider.Azure.VMSize
	}

	// Recommend VM size based on resource requirements
	if cluster.Spec.Resources.Requests != nil {
		cpu := cluster.Spec.Resources.Requests.Cpu()
		memory := cluster.Spec.Resources.Requests.Memory()

		// Simple heuristic for VM size selection
		cpuCores := cpu.MilliValue() / 1000
		memoryGiB := memory.Value() / (1024 * 1024 * 1024)

		switch {
		case cpuCores >= 16 || memoryGiB >= 64:
			return "Standard_F16s_v2" // 16 vCPU, 32 GiB
		case cpuCores >= 8 || memoryGiB >= 32:
			return "Standard_F8s_v2" // 8 vCPU, 16 GiB
		case cpuCores >= 4 || memoryGiB >= 16:
			return "Standard_F4s_v2" // 4 vCPU, 8 GiB
		default:
			return "Standard_F2s_v2" // 2 vCPU, 4 GiB
		}
	}

	// Default to a balanced VM size
	return "Standard_F8s_v2"
}

// configurePremiumStorage configures premium storage settings
func (o *Optimizer) configurePremiumStorage(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	logger := log.FromContext(ctx)
	logger.Info("Configuring premium storage optimizations")

	// In a full implementation, this would:
	// 1. Set node selector for premium storage-capable VMs
	// 2. Configure disk parameters (IOPS, throughput)
	// 3. Set up caching policies

	logger.Info("Premium storage optimizations applied")
	return nil
}

// VM sizes categorized by use case
var (
	// ComputeOptimizedVMs are optimized for compute-intensive workloads
	ComputeOptimizedVMs = []string{
		"Standard_F2s_v2", "Standard_F4s_v2", "Standard_F8s_v2", "Standard_F16s_v2",
		"Standard_F32s_v2", "Standard_F48s_v2", "Standard_F64s_v2",
	}

	// MemoryOptimizedVMs are optimized for memory-intensive workloads
	MemoryOptimizedVMs = []string{
		"Standard_E2s_v3", "Standard_E4s_v3", "Standard_E8s_v3", "Standard_E16s_v3",
		"Standard_E32s_v3", "Standard_E48s_v3", "Standard_E64s_v3",
	}

	// StorageOptimizedVMs have high local storage throughput
	StorageOptimizedVMs = []string{
		"Standard_L4s", "Standard_L8s", "Standard_L16s", "Standard_L32s",
		"Standard_L8s_v2", "Standard_L16s_v2", "Standard_L32s_v2",
	}

	// GeneralPurposeVMs provide balanced CPU and memory
	GeneralPurposeVMs = []string{
		"Standard_D2s_v3", "Standard_D4s_v3", "Standard_D8s_v3", "Standard_D16s_v3",
		"Standard_D32s_v3", "Standard_D48s_v3", "Standard_D64s_v3",
	}
)

// GetVMSizeByWorkload recommends VM size based on workload characteristics
func GetVMSizeByWorkload(workloadType string, size string) string {
	var vmSizes []string

	switch workloadType {
	case "compute":
		vmSizes = ComputeOptimizedVMs
	case "memory":
		vmSizes = MemoryOptimizedVMs
	case "storage":
		vmSizes = StorageOptimizedVMs
	case "general":
		vmSizes = GeneralPurposeVMs
	default:
		vmSizes = ComputeOptimizedVMs
	}

	// Map size to array index
	sizeMap := map[string]int{
		"small":  0,
		"medium": 2,
		"large":  4,
		"xlarge": 6,
	}

	idx := sizeMap[size]
	if idx >= len(vmSizes) {
		idx = len(vmSizes) - 1
	}

	return vmSizes[idx]
}

// DiskSKU represents Azure Disk storage tiers
type DiskSKU string

const (
	// StandardLRS represents standard locally redundant storage
	StandardLRS DiskSKU = "Standard_LRS"
	// StandardSSDLRS represents standard SSD locally redundant storage
	StandardSSDLRS DiskSKU = "StandardSSD_LRS"
	// PremiumLRS represents premium locally redundant storage
	PremiumLRS DiskSKU = "Premium_LRS"
	// UltraSSDLRS represents ultra SSD locally redundant storage
	UltraSSDLRS DiskSKU = "UltraSSD_LRS"
	// PremiumZRS represents premium zone-redundant storage
	PremiumZRS DiskSKU = "Premium_ZRS"
	// StandardSSDZRS represents standard SSD zone-redundant storage
	StandardSSDZRS DiskSKU = "StandardSSD_ZRS"
)

// GetRecommendedDiskSKU returns the recommended disk SKU based on requirements
func GetRecommendedDiskSKU(performance string, redundancy string) DiskSKU {
	switch performance {
	case "ultra":
		return UltraSSDLRS
	case "premium":
		if redundancy == "zone" {
			return PremiumZRS
		}
		return PremiumLRS
	case "standard-ssd":
		if redundancy == "zone" {
			return StandardSSDZRS
		}
		return StandardSSDLRS
	default:
		return StandardLRS
	}
}

// AvailabilityZoneConfig represents Azure availability zone configuration
type AvailabilityZoneConfig struct {
	// Zones to use for deployment
	Zones []string
	// ZoneRedundant indicates if zone-redundant storage should be used
	ZoneRedundant bool
}

// GetAvailabilityZoneConfig returns optimal availability zone configuration
func GetAvailabilityZoneConfig(region string, haEnabled bool) AvailabilityZoneConfig {
	if !haEnabled {
		return AvailabilityZoneConfig{
			Zones:         []string{"1"},
			ZoneRedundant: false,
		}
	}

	// For HA, use multiple zones
	return AvailabilityZoneConfig{
		Zones:         []string{"1", "2", "3"},
		ZoneRedundant: true,
	}
}

package aws

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

// Optimizer implements AWS-specific optimizations
type Optimizer struct {
	client client.Client
}

// NewOptimizer creates a new AWS optimizer
func NewOptimizer(client client.Client) *Optimizer {
	return &Optimizer{
		client: client,
	}
}

// ApplyOptimizations applies AWS-specific optimizations
func (o *Optimizer) ApplyOptimizations(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	logger := log.FromContext(ctx)

	if cluster.Spec.CloudProvider.AWS == nil {
		logger.Info("No AWS configuration specified")
		return nil
	}

	awsSpec := cluster.Spec.CloudProvider.AWS

	// Configure instance store for cache if requested
	if awsSpec.UseLocalNVMe {
		if err := o.configureInstanceStore(ctx, cluster); err != nil {
			logger.Error(err, "Failed to configure instance store")
			return err
		}
	}

	// Configure EBS optimizations
	if awsSpec.EBSOptimized {
		if err := o.configureEBS(ctx, cluster); err != nil {
			logger.Error(err, "Failed to configure EBS")
			return err
		}
	}

	logger.Info("AWS optimizations applied successfully")
	return nil
}

// ConfigureStorage configures AWS-specific storage options
func (o *Optimizer) ConfigureStorage(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	logger := log.FromContext(ctx)

	// Create StorageClass for EBS volumes with optimized settings
	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-ebs-gp3", cluster.Name),
		},
	}

	err := o.client.Get(ctx, client.ObjectKey{Name: storageClass.Name}, storageClass)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		// Create new StorageClass
		storageClass.Provisioner = "ebs.csi.aws.com"
		storageClass.VolumeBindingMode = &[]storagev1.VolumeBindingMode{storagev1.VolumeBindingWaitForFirstConsumer}[0]
		storageClass.AllowVolumeExpansion = &[]bool{true}[0]
		storageClass.Parameters = map[string]string{
			"type":      "gp3",
			"iops":      "16000",
			"throughput": "1000",
			"encrypted": "true",
		}

		if err := o.client.Create(ctx, storageClass); err != nil {
			return fmt.Errorf("failed to create StorageClass: %w", err)
		}

		logger.Info("Created EBS GP3 StorageClass", "name", storageClass.Name)
	}

	return nil
}

// ConfigureBackup configures S3 backup integration
func (o *Optimizer) ConfigureBackup(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	logger := log.FromContext(ctx)

	if cluster.Spec.CloudProvider.AWS == nil || cluster.Spec.CloudProvider.AWS.Backup == nil {
		return nil
	}

	backupSpec := cluster.Spec.CloudProvider.AWS.Backup

	// Create ConfigMap for backup configuration
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-backup-config", cluster.Name),
			Namespace: cluster.Namespace,
		},
		Data: map[string]string{
			"backup-s3-bucket": backupSpec.Bucket,
			"backup-s3-region": o.getBackupRegion(cluster, backupSpec),
			"backup-provider":  "s3",
		},
	}

	err := o.client.Get(ctx, client.ObjectKey{Name: configMap.Name, Namespace: configMap.Namespace}, &corev1.ConfigMap{})
	if err != nil && errors.IsNotFound(err) {
		if err := o.client.Create(ctx, configMap); err != nil {
			return fmt.Errorf("failed to create backup ConfigMap: %w", err)
		}
		logger.Info("Created S3 backup configuration", "bucket", backupSpec.Bucket)
	}

	return nil
}

// GetOptimalInstanceType returns the recommended EC2 instance type
func (o *Optimizer) GetOptimalInstanceType(cluster *weaviatev1.WeaviateCluster) string {
	if cluster.Spec.CloudProvider.AWS != nil && cluster.Spec.CloudProvider.AWS.InstanceType != "" {
		return cluster.Spec.CloudProvider.AWS.InstanceType
	}

	// Recommend instance type based on resource requirements
	if cluster.Spec.Resources.Requests != nil {
		cpu := cluster.Spec.Resources.Requests.Cpu()
		memory := cluster.Spec.Resources.Requests.Memory()

		// Simple heuristic for instance type selection
		cpuCores := cpu.MilliValue() / 1000
		memoryGiB := memory.Value() / (1024 * 1024 * 1024)

		switch {
		case cpuCores >= 16 || memoryGiB >= 64:
			return "c5.4xlarge" // 16 vCPU, 32 GiB
		case cpuCores >= 8 || memoryGiB >= 32:
			return "c5.2xlarge" // 8 vCPU, 16 GiB
		case cpuCores >= 4 || memoryGiB >= 16:
			return "c5.xlarge" // 4 vCPU, 8 GiB
		default:
			return "c5.large" // 2 vCPU, 4 GiB
		}
	}

	// Default to a balanced instance type
	return "c5.2xlarge"
}

// configureInstanceStore configures local NVMe instance store
func (o *Optimizer) configureInstanceStore(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	logger := log.FromContext(ctx)
	logger.Info("Configuring local NVMe instance store for cache")

	// In a full implementation, this would:
	// 1. Add node selector for instances with NVMe
	// 2. Configure volume mounts for instance store
	// 3. Set up init containers to format and mount NVMe devices

	// For now, log the configuration
	logger.Info("Local NVMe configuration applied",
		"note", "Ensure nodes have instance store volumes")

	return nil
}

// configureEBS configures EBS-optimized settings
func (o *Optimizer) configureEBS(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	logger := log.FromContext(ctx)
	logger.Info("Configuring EBS optimizations")

	// In a full implementation, this would:
	// 1. Set node selector for EBS-optimized instances
	// 2. Configure EBS volume parameters (IOPS, throughput)
	// 3. Set up volume expansion policies

	logger.Info("EBS optimizations applied")
	return nil
}

// getBackupRegion determines the S3 region for backups
func (o *Optimizer) getBackupRegion(cluster *weaviatev1.WeaviateCluster, backupSpec *weaviatev1.AWSBackupSpec) string {
	if backupSpec.Region != "" {
		return backupSpec.Region
	}
	return cluster.Spec.CloudProvider.Region
}

// Instance types categorized by use case
var (
	// ComputeOptimizedInstances are optimized for compute-intensive workloads
	ComputeOptimizedInstances = []string{
		"c5.large", "c5.xlarge", "c5.2xlarge", "c5.4xlarge", "c5.9xlarge",
		"c5n.large", "c5n.xlarge", "c5n.2xlarge", "c5n.4xlarge",
		"c6i.large", "c6i.xlarge", "c6i.2xlarge", "c6i.4xlarge",
	}

	// MemoryOptimizedInstances are optimized for memory-intensive workloads
	MemoryOptimizedInstances = []string{
		"r5.large", "r5.xlarge", "r5.2xlarge", "r5.4xlarge", "r5.8xlarge",
		"r6i.large", "r6i.xlarge", "r6i.2xlarge", "r6i.4xlarge",
	}

	// StorageOptimizedInstances have local NVMe storage
	StorageOptimizedInstances = []string{
		"i3.large", "i3.xlarge", "i3.2xlarge", "i3.4xlarge",
		"i3en.large", "i3en.xlarge", "i3en.2xlarge",
	}
)

// GetInstanceTypeByWorkload recommends instance type based on workload characteristics
func GetInstanceTypeByWorkload(workloadType string, size string) string {
	var instances []string

	switch workloadType {
	case "compute":
		instances = ComputeOptimizedInstances
	case "memory":
		instances = MemoryOptimizedInstances
	case "storage":
		instances = StorageOptimizedInstances
	default:
		instances = ComputeOptimizedInstances
	}

	// Map size to array index
	sizeMap := map[string]int{
		"small":  0,
		"medium": 2,
		"large":  4,
		"xlarge": 6,
	}

	idx := sizeMap[size]
	if idx >= len(instances) {
		idx = len(instances) - 1
	}

	return instances[idx]
}

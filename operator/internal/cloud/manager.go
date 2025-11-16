package cloud

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	weaviatev1 "github.com/weaviate/weaviate/operator/api/v1"
	"github.com/weaviate/weaviate/operator/internal/cloud/aws"
	"github.com/weaviate/weaviate/operator/internal/cloud/azure"
	"github.com/weaviate/weaviate/operator/internal/cloud/gcp"
)

// Optimizer defines the interface for cloud provider optimizations
type Optimizer interface {
	// ApplyOptimizations applies cloud-specific optimizations to the cluster
	ApplyOptimizations(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error

	// ConfigureStorage configures cloud-specific storage options
	ConfigureStorage(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error

	// ConfigureBackup configures cloud-specific backup solutions
	ConfigureBackup(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error

	// GetOptimalInstanceType returns the recommended instance/machine type
	GetOptimalInstanceType(cluster *weaviatev1.WeaviateCluster) string
}

// Manager manages cloud provider integrations
type Manager struct {
	client     client.Client
	optimizers map[string]Optimizer
}

// NewManager creates a new cloud provider manager
func NewManager(client client.Client) *Manager {
	return &Manager{
		client: client,
		optimizers: map[string]Optimizer{
			"aws":   aws.NewOptimizer(client),
			"gcp":   gcp.NewOptimizer(client),
			"azure": azure.NewOptimizer(client),
		},
	}
}

// ApplyOptimizations applies cloud provider-specific optimizations
func (m *Manager) ApplyOptimizations(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	logger := log.FromContext(ctx)

	if cluster.Spec.CloudProvider == nil {
		logger.Info("No cloud provider specified, skipping optimizations")
		return nil
	}

	providerType := cluster.Spec.CloudProvider.Type
	optimizer, ok := m.optimizers[providerType]
	if !ok {
		return fmt.Errorf("unsupported cloud provider: %s", providerType)
	}

	logger.Info("Applying cloud optimizations", "provider", providerType)

	// Apply general optimizations
	if err := optimizer.ApplyOptimizations(ctx, cluster); err != nil {
		return fmt.Errorf("failed to apply optimizations: %w", err)
	}

	// Configure storage
	if err := optimizer.ConfigureStorage(ctx, cluster); err != nil {
		return fmt.Errorf("failed to configure storage: %w", err)
	}

	// Configure backup if specified
	if m.hasBackupConfig(cluster) {
		if err := optimizer.ConfigureBackup(ctx, cluster); err != nil {
			return fmt.Errorf("failed to configure backup: %w", err)
		}
	}

	logger.Info("Cloud optimizations applied successfully")
	return nil
}

// GetOptimalInstanceType returns the optimal instance type for the cluster
func (m *Manager) GetOptimalInstanceType(cluster *weaviatev1.WeaviateCluster) (string, error) {
	if cluster.Spec.CloudProvider == nil {
		return "", fmt.Errorf("no cloud provider specified")
	}

	providerType := cluster.Spec.CloudProvider.Type
	optimizer, ok := m.optimizers[providerType]
	if !ok {
		return "", fmt.Errorf("unsupported cloud provider: %s", providerType)
	}

	return optimizer.GetOptimalInstanceType(cluster), nil
}

// hasBackupConfig checks if backup is configured for the cluster
func (m *Manager) hasBackupConfig(cluster *weaviatev1.WeaviateCluster) bool {
	if cluster.Spec.CloudProvider == nil {
		return false
	}

	switch cluster.Spec.CloudProvider.Type {
	case "aws":
		return cluster.Spec.CloudProvider.AWS != nil && cluster.Spec.CloudProvider.AWS.Backup != nil
	case "gcp":
		return cluster.Spec.CloudProvider.GCP != nil && cluster.Spec.CloudProvider.GCP.Backup != nil
	case "azure":
		return cluster.Spec.CloudProvider.Azure != nil && cluster.Spec.CloudProvider.Azure.Backup != nil
	default:
		return false
	}
}

// ValidateConfiguration validates cloud provider configuration
func (m *Manager) ValidateConfiguration(cluster *weaviatev1.WeaviateCluster) error {
	if cluster.Spec.CloudProvider == nil {
		return nil
	}

	providerType := cluster.Spec.CloudProvider.Type
	if _, ok := m.optimizers[providerType]; !ok {
		return fmt.Errorf("unsupported cloud provider: %s (supported: aws, gcp, azure)", providerType)
	}

	// Validate provider-specific configuration
	switch providerType {
	case "aws":
		return m.validateAWSConfig(cluster)
	case "gcp":
		return m.validateGCPConfig(cluster)
	case "azure":
		return m.validateAzureConfig(cluster)
	default:
		return fmt.Errorf("unknown provider type: %s", providerType)
	}
}

// validateAWSConfig validates AWS-specific configuration
func (m *Manager) validateAWSConfig(cluster *weaviatev1.WeaviateCluster) error {
	if cluster.Spec.CloudProvider.AWS == nil {
		return nil
	}

	aws := cluster.Spec.CloudProvider.AWS
	if aws.Backup != nil && aws.Backup.Bucket == "" {
		return fmt.Errorf("AWS backup bucket must be specified")
	}

	return nil
}

// validateGCPConfig validates GCP-specific configuration
func (m *Manager) validateGCPConfig(cluster *weaviatev1.WeaviateCluster) error {
	if cluster.Spec.CloudProvider.GCP == nil {
		return nil
	}

	gcp := cluster.Spec.CloudProvider.GCP
	if gcp.Backup != nil && gcp.Backup.Bucket == "" {
		return fmt.Errorf("GCP backup bucket must be specified")
	}

	return nil
}

// validateAzureConfig validates Azure-specific configuration
func (m *Manager) validateAzureConfig(cluster *weaviatev1.WeaviateCluster) error {
	if cluster.Spec.CloudProvider.Azure == nil {
		return nil
	}

	azure := cluster.Spec.CloudProvider.Azure
	if azure.Backup != nil {
		if azure.Backup.StorageAccount == "" {
			return fmt.Errorf("Azure backup storage account must be specified")
		}
		if azure.Backup.Container == "" {
			return fmt.Errorf("Azure backup container must be specified")
		}
	}

	return nil
}

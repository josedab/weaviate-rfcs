package autoscaler

import (
	"context"
	"fmt"
	"math"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	weaviatev1 "github.com/weaviate/weaviate/operator/api/v1"
	"github.com/weaviate/weaviate/operator/internal/metrics"
)

// AutoScaler manages automatic horizontal pod autoscaling for Weaviate clusters
type AutoScaler struct {
	client  client.Client
	metrics *metrics.Collector
	scaler  *StatefulSetScaler
}

// NewAutoScaler creates a new AutoScaler instance
func NewAutoScaler(client client.Client) *AutoScaler {
	return &AutoScaler{
		client:  client,
		metrics: metrics.NewCollector(client),
		scaler:  NewStatefulSetScaler(client),
	}
}

// Reconcile reconciles auto-scaling for a WeaviateCluster
func (a *AutoScaler) Reconcile(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	logger := log.FromContext(ctx)

	if cluster.Spec.AutoScaling == nil || !cluster.Spec.AutoScaling.Enabled {
		logger.Info("Auto-scaling is not enabled")
		return nil
	}

	spec := cluster.Spec.AutoScaling

	// Create or update HorizontalPodAutoscaler
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, a.client, hpa, func() error {
		// Build metrics
		var metricSpecs []autoscalingv2.MetricSpec

		// CPU-based scaling
		if spec.TargetCPU > 0 {
			targetCPU := int32(spec.TargetCPU)
			metricSpecs = append(metricSpecs, autoscalingv2.MetricSpec{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name: corev1.ResourceCPU,
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: &targetCPU,
					},
				},
			})
		}

		// Memory-based scaling
		if spec.TargetMemory > 0 {
			targetMemory := int32(spec.TargetMemory)
			metricSpecs = append(metricSpecs, autoscalingv2.MetricSpec{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name: corev1.ResourceMemory,
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: &targetMemory,
					},
				},
			})
		}

		// QPS-based scaling (custom metric)
		if spec.TargetQPS != nil {
			targetQPS := *spec.TargetQPS
			metricSpecs = append(metricSpecs, autoscalingv2.MetricSpec{
				Type: autoscalingv2.PodsMetricSourceType,
				Pods: &autoscalingv2.PodsMetricSource{
					Metric: autoscalingv2.MetricIdentifier{
						Name: "weaviate_queries_per_second",
					},
					Target: autoscalingv2.MetricTarget{
						Type:         autoscalingv2.AverageValueMetricType,
						AverageValue: &[]int64{int64(targetQPS)}[0],
					},
				},
			})
		}

		hpa.Spec = autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "StatefulSet",
				Name:       cluster.Name,
			},
			MinReplicas: &spec.MinReplicas,
			MaxReplicas: spec.MaxReplicas,
			Metrics:     metricSpecs,
			Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
				ScaleDown: &autoscalingv2.HPAScalingRules{
					StabilizationWindowSeconds: &[]int32{300}[0], // 5 minutes
					Policies: []autoscalingv2.HPAScalingPolicy{
						{
							Type:          autoscalingv2.PercentScalingPolicy,
							Value:         10,
							PeriodSeconds: 60,
						},
					},
				},
				ScaleUp: &autoscalingv2.HPAScalingRules{
					StabilizationWindowSeconds: &[]int32{0}[0], // No delay
					Policies: []autoscalingv2.HPAScalingPolicy{
						{
							Type:          autoscalingv2.PercentScalingPolicy,
							Value:         50,
							PeriodSeconds: 60,
						},
						{
							Type:          autoscalingv2.PodsScalingPolicy,
							Value:         2,
							PeriodSeconds: 60,
						},
					},
					SelectPolicy: &[]autoscalingv2.ScalingPolicySelect{autoscalingv2.MaxChangePolicySelect}[0],
				},
			},
		}

		return nil
	})

	if err != nil {
		logger.Error(err, "Failed to create or update HPA")
		return err
	}

	logger.Info("Auto-scaling reconciled successfully")
	return nil
}

// Scale manually scales the cluster (used for custom metrics)
func (a *AutoScaler) Scale(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	logger := log.FromContext(ctx)

	if cluster.Spec.AutoScaling == nil || !cluster.Spec.AutoScaling.Enabled {
		return nil
	}

	// Collect current metrics
	clusterMetrics, err := a.metrics.Collect(ctx, cluster)
	if err != nil {
		logger.Error(err, "Failed to collect metrics")
		return err
	}

	// Calculate desired replicas
	desired := a.calculateDesiredReplicas(cluster, clusterMetrics)

	// Check if scaling is needed
	current := cluster.Status.Replicas
	if desired == current {
		logger.Info("No scaling needed", "current", current, "desired", desired)
		return nil
	}

	logger.Info("Scaling cluster", "from", current, "to", desired)

	// Scale StatefulSet
	if err := a.scaler.Scale(ctx, cluster, desired); err != nil {
		return err
	}

	// Wait for new pods to be ready
	if err := a.waitForReady(ctx, cluster, desired); err != nil {
		return err
	}

	logger.Info("Scaling completed successfully")
	return nil
}

// calculateDesiredReplicas calculates the desired number of replicas based on metrics
func (a *AutoScaler) calculateDesiredReplicas(
	cluster *weaviatev1.WeaviateCluster,
	clusterMetrics *metrics.ClusterMetrics,
) int32 {
	spec := cluster.Spec.AutoScaling
	current := cluster.Status.Replicas

	if current == 0 {
		current = 1
	}

	var desiredReplicas int32 = current

	// CPU-based scaling
	if spec.TargetCPU > 0 && clusterMetrics.CPUUsage > 0 {
		cpuReplicas := int32(math.Ceil(
			float64(clusterMetrics.CPUUsage) / float64(spec.TargetCPU) * float64(current),
		))
		if cpuReplicas > desiredReplicas {
			desiredReplicas = cpuReplicas
		}
	}

	// Memory-based scaling
	if spec.TargetMemory > 0 && clusterMetrics.MemoryUsage > 0 {
		memReplicas := int32(math.Ceil(
			float64(clusterMetrics.MemoryUsage) / float64(spec.TargetMemory) * float64(current),
		))
		if memReplicas > desiredReplicas {
			desiredReplicas = memReplicas
		}
	}

	// QPS-based scaling
	if spec.TargetQPS != nil && clusterMetrics.QPS > 0 {
		qpsReplicas := int32(math.Ceil(
			float64(clusterMetrics.QPS) / float64(*spec.TargetQPS),
		))
		if qpsReplicas > desiredReplicas {
			desiredReplicas = qpsReplicas
		}
	}

	// Apply min/max constraints
	if desiredReplicas < spec.MinReplicas {
		desiredReplicas = spec.MinReplicas
	}
	if desiredReplicas > spec.MaxReplicas {
		desiredReplicas = spec.MaxReplicas
	}

	return desiredReplicas
}

// waitForReady waits for the cluster to reach the desired number of ready replicas
func (a *AutoScaler) waitForReady(ctx context.Context, cluster *weaviatev1.WeaviateCluster, desired int32) error {
	logger := log.FromContext(ctx)
	timeout := time.After(10 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for replicas to be ready")
		case <-ticker.C:
			statefulSet := &appsv1.StatefulSet{}
			err := a.client.Get(ctx, client.ObjectKey{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			}, statefulSet)
			if err != nil {
				if errors.IsNotFound(err) {
					continue
				}
				return err
			}

			if statefulSet.Status.ReadyReplicas >= desired {
				logger.Info("All replicas are ready", "ready", statefulSet.Status.ReadyReplicas)
				return nil
			}

			logger.Info("Waiting for replicas to be ready",
				"ready", statefulSet.Status.ReadyReplicas,
				"desired", desired)
		}
	}
}

// StatefulSetScaler handles scaling of StatefulSets
type StatefulSetScaler struct {
	client client.Client
}

// NewStatefulSetScaler creates a new StatefulSetScaler
func NewStatefulSetScaler(client client.Client) *StatefulSetScaler {
	return &StatefulSetScaler{client: client}
}

// Scale scales a StatefulSet to the desired number of replicas
func (s *StatefulSetScaler) Scale(ctx context.Context, cluster *weaviatev1.WeaviateCluster, replicas int32) error {
	logger := log.FromContext(ctx)

	statefulSet := &appsv1.StatefulSet{}
	err := s.client.Get(ctx, client.ObjectKey{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}, statefulSet)
	if err != nil {
		return err
	}

	if statefulSet.Spec.Replicas != nil && *statefulSet.Spec.Replicas == replicas {
		logger.Info("StatefulSet already at desired replicas", "replicas", replicas)
		return nil
	}

	statefulSet.Spec.Replicas = &replicas
	if err := s.client.Update(ctx, statefulSet); err != nil {
		return err
	}

	logger.Info("StatefulSet scaled", "replicas", replicas)
	return nil
}

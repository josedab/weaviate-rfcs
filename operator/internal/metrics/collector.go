package metrics

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	weaviatev1 "github.com/weaviate/weaviate/operator/api/v1"
)

// ClusterMetrics contains metrics for a Weaviate cluster
type ClusterMetrics struct {
	// CPUUsage is the average CPU utilization percentage across all pods
	CPUUsage int32

	// MemoryUsage is the average memory utilization percentage across all pods
	MemoryUsage int32

	// QPS is the total queries per second across all pods
	QPS int32

	// Latency is the average query latency
	Latency string

	// PodMetrics contains per-pod metrics
	PodMetrics []PodMetrics
}

// PodMetrics contains metrics for a single pod
type PodMetrics struct {
	// Name is the pod name
	Name string

	// CPUUsage is CPU utilization percentage
	CPUUsage int32

	// MemoryUsage is memory utilization percentage
	MemoryUsage int32

	// Ready indicates if the pod is ready
	Ready bool
}

// Collector collects metrics from Weaviate clusters
type Collector struct {
	client client.Client
}

// NewCollector creates a new metrics collector
func NewCollector(client client.Client) *Collector {
	return &Collector{
		client: client,
	}
}

// Collect collects current metrics for a cluster
func (c *Collector) Collect(ctx context.Context, cluster *weaviatev1.WeaviateCluster) (*ClusterMetrics, error) {
	logger := log.FromContext(ctx)

	// List pods for this cluster
	podList := &corev1.PodList{}
	labels := map[string]string{
		"weaviate.io/cluster": cluster.Name,
	}
	err := c.client.List(ctx, podList, client.InNamespace(cluster.Namespace), client.MatchingLabels(labels))
	if err != nil {
		return nil, err
	}

	if len(podList.Items) == 0 {
		logger.Info("No pods found for cluster")
		return &ClusterMetrics{}, nil
	}

	// Collect pod-level metrics
	var podMetrics []PodMetrics
	var totalCPU, totalMemory int32
	readyPods := 0

	for _, pod := range podList.Items {
		pm := c.collectPodMetrics(ctx, &pod)
		podMetrics = append(podMetrics, pm)

		if pm.Ready {
			totalCPU += pm.CPUUsage
			totalMemory += pm.MemoryUsage
			readyPods++
		}
	}

	// Calculate averages
	avgCPU := int32(0)
	avgMemory := int32(0)
	if readyPods > 0 {
		avgCPU = totalCPU / int32(readyPods)
		avgMemory = totalMemory / int32(readyPods)
	}

	// Collect QPS (this would typically come from Prometheus or custom metrics)
	qps := c.collectQPS(ctx, cluster)

	return &ClusterMetrics{
		CPUUsage:    avgCPU,
		MemoryUsage: avgMemory,
		QPS:         qps,
		PodMetrics:  podMetrics,
	}, nil
}

// collectPodMetrics collects metrics for a single pod
func (c *Collector) collectPodMetrics(ctx context.Context, pod *corev1.Pod) PodMetrics {
	// Check if pod is ready
	ready := false
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			ready = true
			break
		}
	}

	// Calculate resource usage
	// Note: In a real implementation, this would query the metrics server
	// For now, we'll use placeholder values
	cpuUsage := c.estimateCPUUsage(pod)
	memoryUsage := c.estimateMemoryUsage(pod)

	return PodMetrics{
		Name:        pod.Name,
		CPUUsage:    cpuUsage,
		MemoryUsage: memoryUsage,
		Ready:       ready,
	}
}

// estimateCPUUsage estimates CPU usage based on pod specs
// In production, this would query the metrics server API
func (c *Collector) estimateCPUUsage(pod *corev1.Pod) int32 {
	// This is a placeholder implementation
	// In production, you would query metrics.k8s.io API or Prometheus
	if len(pod.Spec.Containers) == 0 {
		return 0
	}

	// Return a default value for now
	// Real implementation would fetch from metrics server
	return 50 // 50% as placeholder
}

// estimateMemoryUsage estimates memory usage based on pod specs
// In production, this would query the metrics server API
func (c *Collector) estimateMemoryUsage(pod *corev1.Pod) int32 {
	// This is a placeholder implementation
	// In production, you would query metrics.k8s.io API or Prometheus
	if len(pod.Spec.Containers) == 0 {
		return 0
	}

	// Return a default value for now
	// Real implementation would fetch from metrics server
	return 60 // 60% as placeholder
}

// collectQPS collects queries per second metrics
// In production, this would query Prometheus or custom metrics API
func (c *Collector) collectQPS(ctx context.Context, cluster *weaviatev1.WeaviateCluster) int32 {
	// This is a placeholder implementation
	// In production, you would query Prometheus with a query like:
	// sum(rate(weaviate_queries_total[1m]))

	// Return a default value for now
	return 100 // 100 QPS as placeholder
}

// GetResourceUsage calculates resource usage percentage
func GetResourceUsage(used, requested resource.Quantity) int32 {
	if requested.IsZero() {
		return 0
	}

	usedValue := used.MilliValue()
	requestedValue := requested.MilliValue()

	if requestedValue == 0 {
		return 0
	}

	percentage := (usedValue * 100) / requestedValue
	return int32(percentage)
}

// MetricsServerClient provides access to Kubernetes metrics server
type MetricsServerClient struct {
	client client.Client
}

// NewMetricsServerClient creates a new metrics server client
func NewMetricsServerClient(client client.Client) *MetricsServerClient {
	return &MetricsServerClient{
		client: client,
	}
}

// GetPodMetrics retrieves metrics for a specific pod from metrics server
// Note: This requires metrics-server to be installed in the cluster
func (m *MetricsServerClient) GetPodMetrics(ctx context.Context, namespace, name string) (*PodMetrics, error) {
	// In a full implementation, this would use the metrics.k8s.io API
	// For now, return a placeholder
	return &PodMetrics{
		Name:        name,
		CPUUsage:    50,
		MemoryUsage: 60,
		Ready:       true,
	}, nil
}

// PrometheusClient provides access to Prometheus metrics
type PrometheusClient struct {
	endpoint string
}

// NewPrometheusClient creates a new Prometheus client
func NewPrometheusClient(endpoint string) *PrometheusClient {
	return &PrometheusClient{
		endpoint: endpoint,
	}
}

// QueryQPS queries Prometheus for QPS metrics
func (p *PrometheusClient) QueryQPS(ctx context.Context, cluster *weaviatev1.WeaviateCluster) (int32, error) {
	// In a full implementation, this would execute a PromQL query
	// Example query: sum(rate(weaviate_queries_total{cluster="production"}[1m]))

	// For now, return a placeholder
	return 100, nil
}

// QueryLatency queries Prometheus for latency metrics
func (p *PrometheusClient) QueryLatency(ctx context.Context, cluster *weaviatev1.WeaviateCluster) (string, error) {
	// In a full implementation, this would execute a PromQL query
	// Example query: histogram_quantile(0.99, rate(weaviate_query_duration_seconds_bucket[5m]))

	// For now, return a placeholder
	return "50ms", nil
}

// Custom metric for HPA
type CustomMetric struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Timestamp metav1.Time     `json:"timestamp"`
	Window    metav1.Duration `json:"window,omitempty"`
	Value     resource.Quantity `json:"value"`
}

// PublishCustomMetric publishes a custom metric for HPA consumption
func PublishCustomMetric(ctx context.Context, name string, value int64) error {
	// In production, this would publish to the custom metrics API
	// For now, this is a placeholder
	logger := log.FromContext(ctx)
	logger.Info("Publishing custom metric", "name", name, "value", value)
	return nil
}

// WatchMetrics continuously monitors and publishes metrics
func (c *Collector) WatchMetrics(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	logger := log.FromContext(ctx)
	logger.Info("Starting metrics watch for cluster", "cluster", cluster.Name)

	// In production, this would run in a loop, collecting and publishing metrics
	// For demonstration purposes, we'll just show the structure

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			metrics, err := c.Collect(ctx, cluster)
			if err != nil {
				logger.Error(err, "Failed to collect metrics")
				continue
			}

			// Publish custom metrics for HPA
			if cluster.Spec.AutoScaling != nil && cluster.Spec.AutoScaling.TargetQPS != nil {
				if err := PublishCustomMetric(ctx, "weaviate_queries_per_second", int64(metrics.QPS)); err != nil {
					logger.Error(err, "Failed to publish QPS metric")
				}
			}

			// Log current metrics
			logger.V(1).Info("Current metrics",
				"cpu", fmt.Sprintf("%d%%", metrics.CPUUsage),
				"memory", fmt.Sprintf("%d%%", metrics.MemoryUsage),
				"qps", metrics.QPS)
		}
	}
}

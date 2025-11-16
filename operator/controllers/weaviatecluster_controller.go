package controllers

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	weaviatev1 "github.com/weaviate/weaviate/operator/api/v1"
	"github.com/weaviate/weaviate/operator/internal/autoscaler"
	"github.com/weaviate/weaviate/operator/internal/cloud"
)

const (
	finalizerName = "weaviate.io/finalizer"
)

// WeaviateClusterReconciler reconciles a WeaviateCluster object
type WeaviateClusterReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	AutoScaler *autoscaler.AutoScaler
	CloudMgr   *cloud.Manager
}

// +kubebuilder:rbac:groups=weaviate.io,resources=weaviateclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=weaviate.io,resources=weaviateclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=weaviate.io,resources=weaviateclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *WeaviateClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the WeaviateCluster instance
	cluster := &weaviatev1.WeaviateCluster{}
	if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("WeaviateCluster resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get WeaviateCluster")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !cluster.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, cluster)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(cluster, finalizerName) {
		controllerutil.AddFinalizer(cluster, finalizerName)
		if err := r.Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Reconcile StatefulSet
	if err := r.reconcileStatefulSet(ctx, cluster); err != nil {
		logger.Error(err, "Failed to reconcile StatefulSet")
		r.updateStatus(ctx, cluster, "Error", err.Error())
		return ctrl.Result{}, err
	}

	// Reconcile Services
	if err := r.reconcileServices(ctx, cluster); err != nil {
		logger.Error(err, "Failed to reconcile Services")
		r.updateStatus(ctx, cluster, "Error", err.Error())
		return ctrl.Result{}, err
	}

	// Reconcile Storage
	if err := r.reconcileStorage(ctx, cluster); err != nil {
		logger.Error(err, "Failed to reconcile Storage")
		r.updateStatus(ctx, cluster, "Error", err.Error())
		return ctrl.Result{}, err
	}

	// Apply cloud provider optimizations
	if cluster.Spec.CloudProvider != nil {
		if err := r.CloudMgr.ApplyOptimizations(ctx, cluster); err != nil {
			logger.Error(err, "Failed to apply cloud optimizations")
			// Continue despite cloud optimization errors
		}
	}

	// Reconcile Auto-scaling
	if cluster.Spec.AutoScaling != nil && cluster.Spec.AutoScaling.Enabled {
		if err := r.AutoScaler.Reconcile(ctx, cluster); err != nil {
			logger.Error(err, "Failed to reconcile auto-scaling")
			// Continue despite auto-scaling errors
		}
	}

	// Update status
	if err := r.updateStatus(ctx, cluster, "Running", ""); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue after 30 seconds for periodic reconciliation
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// reconcileStatefulSet creates or updates the StatefulSet for Weaviate
func (r *WeaviateClusterReconciler) reconcileStatefulSet(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	logger := log.FromContext(ctx)

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, statefulSet, func() error {
		// Set labels
		labels := map[string]string{
			"app":                          "weaviate",
			"weaviate.io/cluster":          cluster.Name,
			"app.kubernetes.io/name":       "weaviate",
			"app.kubernetes.io/instance":   cluster.Name,
			"app.kubernetes.io/managed-by": "weaviate-operator",
		}

		statefulSet.Spec = appsv1.StatefulSetSpec{
			Replicas:    &cluster.Spec.Replicas,
			ServiceName: cluster.Name + "-headless",
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "weaviate",
							Image: r.getWeaviateImage(cluster),
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "grpc",
									ContainerPort: 50051,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: r.buildEnvVars(cluster),
							Resources: cluster.Spec.Resources,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/var/lib/weaviate",
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/v1/.well-known/live",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
								TimeoutSeconds:      3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/v1/.well-known/ready",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
								TimeoutSeconds:      3,
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						StorageClassName: &cluster.Spec.Storage.StorageClassName,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(cluster.Spec.Storage.Size),
							},
						},
					},
				},
			},
		}

		// Set owner reference
		return controllerutil.SetControllerReference(cluster, statefulSet, r.Scheme)
	})

	if err != nil {
		logger.Error(err, "Failed to create or update StatefulSet")
		return err
	}

	logger.Info("StatefulSet reconciled successfully")
	return nil
}

// reconcileServices creates headless and regular services for Weaviate
func (r *WeaviateClusterReconciler) reconcileServices(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	logger := log.FromContext(ctx)

	labels := map[string]string{
		"app":                 "weaviate",
		"weaviate.io/cluster": cluster.Name,
	}

	// Headless service for StatefulSet
	headlessService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name + "-headless",
			Namespace: cluster.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, headlessService, func() error {
		headlessService.Spec = corev1.ServiceSpec{
			ClusterIP: "None",
			Selector:  labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "grpc",
					Port:       50051,
					TargetPort: intstr.FromInt(50051),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		}
		return controllerutil.SetControllerReference(cluster, headlessService, r.Scheme)
	})

	if err != nil {
		return err
	}

	// Regular service for external access
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		service.Spec = corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "grpc",
					Port:       50051,
					TargetPort: intstr.FromInt(50051),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		}
		return controllerutil.SetControllerReference(cluster, service, r.Scheme)
	})

	if err != nil {
		return err
	}

	logger.Info("Services reconciled successfully")
	return nil
}

// reconcileStorage handles storage configuration
func (r *WeaviateClusterReconciler) reconcileStorage(ctx context.Context, cluster *weaviatev1.WeaviateCluster) error {
	// Storage is managed via VolumeClaimTemplates in StatefulSet
	// Additional storage logic can be added here for cloud-specific optimizations
	return nil
}

// handleDeletion handles cleanup when cluster is deleted
func (r *WeaviateClusterReconciler) handleDeletion(ctx context.Context, cluster *weaviatev1.WeaviateCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(cluster, finalizerName) {
		// Perform cleanup
		logger.Info("Performing cleanup for WeaviateCluster")

		// Remove finalizer
		controllerutil.RemoveFinalizer(cluster, finalizerName)
		if err := r.Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// updateStatus updates the cluster status
func (r *WeaviateClusterReconciler) updateStatus(ctx context.Context, cluster *weaviatev1.WeaviateCluster, phase, message string) error {
	cluster.Status.Phase = phase
	cluster.Status.ObservedGeneration = cluster.Generation

	// Get current replica counts
	statefulSet := &appsv1.StatefulSet{}
	err := r.Get(ctx, client.ObjectKey{Name: cluster.Name, Namespace: cluster.Namespace}, statefulSet)
	if err == nil {
		cluster.Status.Replicas = statefulSet.Status.Replicas
		cluster.Status.ReadyReplicas = statefulSet.Status.ReadyReplicas
	}

	// Update conditions
	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             phase,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	if phase == "Error" {
		condition.Status = metav1.ConditionFalse
	}

	// Update or append condition
	found := false
	for i, c := range cluster.Status.Conditions {
		if c.Type == condition.Type {
			cluster.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		cluster.Status.Conditions = append(cluster.Status.Conditions, condition)
	}

	return r.Status().Update(ctx, cluster)
}

// getWeaviateImage returns the container image for Weaviate
func (r *WeaviateClusterReconciler) getWeaviateImage(cluster *weaviatev1.WeaviateCluster) string {
	if cluster.Spec.Image != nil && cluster.Spec.Image.Repository != "" {
		repo := cluster.Spec.Image.Repository
		tag := cluster.Spec.Image.Tag
		if tag == "" {
			tag = cluster.Spec.Version
		}
		return fmt.Sprintf("%s:%s", repo, tag)
	}
	return fmt.Sprintf("semitechnologies/weaviate:%s", cluster.Spec.Version)
}

// buildEnvVars constructs environment variables for Weaviate container
func (r *WeaviateClusterReconciler) buildEnvVars(cluster *weaviatev1.WeaviateCluster) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{
			Name:  "QUERY_DEFAULTS_LIMIT",
			Value: "25",
		},
		{
			Name:  "AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED",
			Value: "true",
		},
		{
			Name:  "PERSISTENCE_DATA_PATH",
			Value: "/var/lib/weaviate",
		},
		{
			Name:  "DEFAULT_VECTORIZER_MODULE",
			Value: "none",
		},
		{
			Name:  "CLUSTER_HOSTNAME",
			Value: "$(POD_NAME)." + cluster.Name + "-headless",
		},
	}

	// Add pod name for clustering
	envVars = append(envVars, corev1.EnvVar{
		Name: "POD_NAME",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.name",
			},
		},
	})

	// Add custom environment variables
	if cluster.Spec.Env != nil {
		envVars = append(envVars, cluster.Spec.Env...)
	}

	// Add module-specific environment variables
	for _, module := range cluster.Spec.Modules {
		if module.Enabled {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "ENABLE_MODULES",
				Value: module.Name,
			})
		}
	}

	return envVars
}

// SetupWithManager sets up the controller with the Manager
func (r *WeaviateClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&weaviatev1.WeaviateCluster{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

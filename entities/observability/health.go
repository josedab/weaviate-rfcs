//                           _       _
// __      _____  __ ___   ___  __ _| |_ ___
// \ \ /\ / / _ \/ _` \ \ / / |/ _` | __/ _ \
//  \ V  V /  __/ (_| |\ V /| | (_| | ||  __/
//   \_/\_/ \___|\__,_| \_/ |_|\__,_|\__\___|
//
//  Copyright © 2016 - 2025 Weaviate B.V. All rights reserved.
//
//  CONTACT: hello@weaviate.io
//

package observability

// HealthStatus represents overall health status
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// IndexHealthResponse represents the health check response for an index
type IndexHealthResponse struct {
	Status HealthStatus        `json:"status"`
	Checks IndexHealthChecks   `json:"checks"`
}

// IndexHealthChecks contains all health check results
type IndexHealthChecks struct {
	Connectivity      ConnectivityCheck      `json:"connectivity"`
	LayerDistribution LayerDistributionCheck `json:"layer_distribution"`
	EntryPoint        EntryPointCheck        `json:"entry_point"`
	Tombstones        TombstonesCheck        `json:"tombstones"`
}

// ConnectivityCheck represents graph connectivity health
type ConnectivityCheck struct {
	Status             HealthStatus `json:"status"`
	UnreachableNodes   int          `json:"unreachable_nodes"`
	IsolatedComponents int          `json:"isolated_components"`
}

// LayerDistributionCheck represents layer distribution health
type LayerDistributionCheck struct {
	Status               HealthStatus   `json:"status"`
	Layers               map[string]int `json:"layers"` // layer number -> node count
	ExpectedDistribution string         `json:"expected_distribution"`
}

// EntryPointCheck represents entry point health
type EntryPointCheck struct {
	Status HealthStatus `json:"status"`
	ID     uint64       `json:"id"`
	Degree int          `json:"degree"`
	Layer  int          `json:"layer"`
}

// TombstonesCheck represents tombstone health
type TombstonesCheck struct {
	Status     HealthStatus `json:"status"`
	Count      int          `json:"count"`
	Percentage float64      `json:"percentage"`
	Threshold  float64      `json:"threshold"`
}

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

package hnsw

import (
	"fmt"

	"github.com/weaviate/weaviate/entities/observability"
)

// ComputeHealthMetrics computes and updates health metrics for the HNSW index
func (h *hnsw) ComputeHealthMetrics() {
	if h.metrics == nil || !h.metrics.enabled {
		return
	}

	h.shardedNodeLocks.RLock(h.currentMaximumLayer)
	maxLayer := h.currentMaximumLayer
	entryPointID := h.entryPointID
	h.shardedNodeLocks.RUnlock(h.currentMaximumLayer)

	// Update max layer
	h.metrics.SetMaxLayer(maxLayer)

	// Update entry point metrics
	h.metrics.SetEntrypointID(entryPointID)

	// Compute layer distribution
	layerCounts := make(map[int]int)
	avgDegrees := make(map[int]float64)
	maxDegrees := make(map[int]int)

	// Compute unreachable nodes and isolated components
	unreachableNodes := 0
	isolatedComponents := 0

	// Iterate through all layers
	for layer := 0; layer <= maxLayer; layer++ {
		h.shardedNodeLocks.RLock(layer)

		nodeCount := 0
		totalDegree := 0
		layerMaxDegree := 0

		// Count nodes and connections in this layer
		for i := uint64(0); i < uint64(h.nodes.Len()); i++ {
			node := h.nodes.Get(i)
			if node == nil {
				continue
			}

			// Check if node exists at this layer
			if int(node.level) >= layer {
				nodeCount++

				// Get connections at this layer
				connections := node.connectionsAtLevel(layer)
				degree := len(connections)
				totalDegree += degree

				if degree > layerMaxDegree {
					layerMaxDegree = degree
				}
			}
		}

		h.shardedNodeLocks.RUnlock(layer)

		layerCounts[layer] = nodeCount
		maxDegrees[layer] = layerMaxDegree

		if nodeCount > 0 {
			avgDegrees[layer] = float64(totalDegree) / float64(nodeCount)
		} else {
			avgDegrees[layer] = 0
		}

		// Update metrics for this layer
		h.metrics.SetLayerNodeCount(layer, nodeCount)
		h.metrics.SetAvgDegree(layer, avgDegrees[layer])
		h.metrics.SetMaxDegree(layer, maxDegrees[layer])
	}

	// Get entry point degree
	if entryPointID < uint64(h.nodes.Len()) {
		h.shardedNodeLocks.RLock(h.currentMaximumLayer)
		entryNode := h.nodes.Get(entryPointID)
		if entryNode != nil {
			degree := len(entryNode.connectionsAtLevel(h.currentMaximumLayer))
			h.metrics.SetEntrypointDegree(degree)
		}
		h.shardedNodeLocks.RUnlock(h.currentMaximumLayer)
	}

	// TODO: Implement graph traversal to detect unreachable nodes
	// This would require a breadth-first search from the entry point
	// For now, we'll set to 0
	h.metrics.SetUnreachableNodes(unreachableNodes)
	h.metrics.SetIsolatedComponents(isolatedComponents)
}

// GetHealthCheck returns the health check status for this HNSW index
func (h *hnsw) GetHealthCheck(totalObjects int) observability.IndexHealthResponse {
	h.shardedNodeLocks.RLock(h.currentMaximumLayer)
	maxLayer := h.currentMaximumLayer
	entryPointID := h.entryPointID
	h.shardedNodeLocks.RUnlock(h.currentMaximumLayer)

	// Compute layer distribution
	layers := make(map[string]int)
	for layer := 0; layer <= maxLayer; layer++ {
		h.shardedNodeLocks.RLock(layer)

		nodeCount := 0
		for i := uint64(0); i < uint64(h.nodes.Len()); i++ {
			node := h.nodes.Get(i)
			if node != nil && int(node.level) >= layer {
				nodeCount++
			}
		}

		h.shardedNodeLocks.RUnlock(layer)
		layers[fmt.Sprintf("%d", layer)] = nodeCount
	}

	// Get entry point degree
	entryDegree := 0
	if entryPointID < uint64(h.nodes.Len()) {
		h.shardedNodeLocks.RLock(h.currentMaximumLayer)
		entryNode := h.nodes.Get(entryPointID)
		if entryNode != nil {
			entryDegree = len(entryNode.connectionsAtLevel(h.currentMaximumLayer))
		}
		h.shardedNodeLocks.RUnlock(h.currentMaximumLayer)
	}

	// Get tombstone count
	tombstoneCount := h.tombstones.Len()
	tombstonePercentage := 0.0
	if totalObjects > 0 {
		tombstonePercentage = float64(tombstoneCount) / float64(totalObjects) * 100.0
	}

	// Determine tombstone status
	tombstoneStatus := observability.HealthStatusHealthy
	tombstoneThreshold := 10.0 // 10% threshold
	if tombstonePercentage > tombstoneThreshold {
		tombstoneStatus = observability.HealthStatusUnhealthy
	} else if tombstonePercentage > 5.0 {
		tombstoneStatus = observability.HealthStatusDegraded
	}

	// TODO: Implement connectivity check
	connectivityStatus := observability.HealthStatusHealthy
	unreachableNodes := 0
	isolatedComponents := 0

	// Determine overall status
	overallStatus := observability.HealthStatusHealthy
	if tombstoneStatus == observability.HealthStatusUnhealthy || connectivityStatus == observability.HealthStatusUnhealthy {
		overallStatus = observability.HealthStatusUnhealthy
	} else if tombstoneStatus == observability.HealthStatusDegraded || connectivityStatus == observability.HealthStatusDegraded {
		overallStatus = observability.HealthStatusDegraded
	}

	return observability.IndexHealthResponse{
		Status: overallStatus,
		Checks: observability.IndexHealthChecks{
			Connectivity: observability.ConnectivityCheck{
				Status:             connectivityStatus,
				UnreachableNodes:   unreachableNodes,
				IsolatedComponents: isolatedComponents,
			},
			LayerDistribution: observability.LayerDistributionCheck{
				Status:               observability.HealthStatusHealthy,
				Layers:               layers,
				ExpectedDistribution: "normal",
			},
			EntryPoint: observability.EntryPointCheck{
				Status: observability.HealthStatusHealthy,
				ID:     entryPointID,
				Degree: entryDegree,
				Layer:  maxLayer,
			},
			Tombstones: observability.TombstonesCheck{
				Status:     tombstoneStatus,
				Count:      tombstoneCount,
				Percentage: tombstonePercentage,
				Threshold:  tombstoneThreshold,
			},
		},
	}
}

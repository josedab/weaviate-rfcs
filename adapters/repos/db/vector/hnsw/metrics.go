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
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaviate/weaviate/usecases/monitoring"
)

type Metrics struct {
	enabled                       bool
	tombstones                    prometheus.Gauge
	threads                       prometheus.Gauge
	insert                        prometheus.Gauge
	insertTime                    prometheus.ObserverVec
	delete                        prometheus.Gauge
	deleteTime                    prometheus.ObserverVec
	cleaned                       prometheus.Counter
	size                          prometheus.Gauge
	grow                          prometheus.Observer
	startupProgress               prometheus.Gauge
	startupDurations              prometheus.ObserverVec
	startupDiskIO                 prometheus.ObserverVec
	tombstoneReassignNeighbors    prometheus.Counter
	tombstoneFindGlobalEntrypoint prometheus.Counter
	tombstoneFindLocalEntrypoint  prometheus.Counter
	tombstoneDeleteListSize       prometheus.Gauge
	tombstoneUnexpected           prometheus.CounterVec
	tombstoneStart                prometheus.Gauge
	tombstoneEnd                  prometheus.Gauge
	tombstoneProgress             prometheus.Gauge

	// HNSW Health Metrics (RFC 03: Enhanced Observability Suite)
	unreachableNodes      prometheus.Gauge
	isolatedComponents    prometheus.Gauge
	avgDegree             *prometheus.GaugeVec
	maxDegree             *prometheus.GaugeVec
	layerNodeCount        *prometheus.GaugeVec
	maxLayer              prometheus.Gauge
	entrypointID          prometheus.Gauge
	entrypointDegree      prometheus.Gauge
	entrypointChanges     prometheus.Counter
}

func NewMetrics(prom *monitoring.PrometheusMetrics,
	className, shardName string,
) *Metrics {
	if prom == nil {
		return &Metrics{enabled: false}
	}

	if prom.Group {
		className = "n/a"
		shardName = "n/a"
	}

	tombstones := prom.VectorIndexTombstones.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	threads := prom.VectorIndexTombstoneCleanupThreads.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	cleaned := prom.VectorIndexTombstoneCleanedCount.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	insert := prom.VectorIndexOperations.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
		"operation":  "create",
	})

	insertTime := prom.VectorIndexDurations.MustCurryWith(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
		"operation":  "create",
	})

	del := prom.VectorIndexOperations.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
		"operation":  "delete",
	})

	deleteTime := prom.VectorIndexDurations.MustCurryWith(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
		"operation":  "delete",
	})

	size := prom.VectorIndexSize.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	grow := prom.VectorIndexMaintenanceDurations.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
		"operation":  "grow",
	})

	startupProgress := prom.StartupProgress.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
		"operation":  "hnsw_read_commitlogs",
	})

	startupDurations := prom.StartupDurations.MustCurryWith(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	startupDiskIO := prom.StartupDiskIO.MustCurryWith(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	tombstoneReassignNeighbors := prom.TombstoneReassignNeighbors.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	tombstoneUnexpected := prom.VectorIndexTombstoneUnexpected.MustCurryWith(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	tombstoneStart := prom.VectorIndexTombstoneCycleStart.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	tombstoneEnd := prom.VectorIndexTombstoneCycleEnd.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	tombstoneProgress := prom.VectorIndexTombstoneCycleProgress.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	tombstoneFindGlobalEntrypoint := prom.TombstoneFindGlobalEntrypoint.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	tombstoneFindLocalEntrypoint := prom.TombstoneFindLocalEntrypoint.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	tombstoneDeleteListSize := prom.TombstoneDeleteListSize.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	// HNSW Health Metrics (RFC 03: Enhanced Observability Suite)
	unreachableNodes := prom.VectorIndexUnreachableNodes.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	isolatedComponents := prom.VectorIndexIsolatedComponents.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	avgDegree := prom.VectorIndexAvgDegree.MustCurryWith(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	maxDegree := prom.VectorIndexMaxDegree.MustCurryWith(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	layerNodeCount := prom.VectorIndexLayerNodeCount.MustCurryWith(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	maxLayer := prom.VectorIndexMaxLayer.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	entrypointID := prom.VectorIndexEntrypointID.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	entrypointDegree := prom.VectorIndexEntrypointDegree.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	entrypointChanges := prom.VectorIndexEntrypointChanges.With(prometheus.Labels{
		"class_name": className,
		"shard_name": shardName,
	})

	return &Metrics{
		enabled:                       true,
		tombstones:                    tombstones,
		threads:                       threads,
		cleaned:                       cleaned,
		insert:                        insert,
		insertTime:                    insertTime,
		delete:                        del,
		deleteTime:                    deleteTime,
		size:                          size,
		grow:                          grow,
		startupProgress:               startupProgress,
		startupDurations:              startupDurations,
		startupDiskIO:                 startupDiskIO,
		tombstoneReassignNeighbors:    tombstoneReassignNeighbors,
		tombstoneFindGlobalEntrypoint: tombstoneFindGlobalEntrypoint,
		tombstoneFindLocalEntrypoint:  tombstoneFindLocalEntrypoint,
		tombstoneDeleteListSize:       tombstoneDeleteListSize,
		tombstoneUnexpected:           *tombstoneUnexpected,
		tombstoneStart:                tombstoneStart,
		tombstoneEnd:                  tombstoneEnd,
		tombstoneProgress:             tombstoneProgress,
		// HNSW Health Metrics
		unreachableNodes:   unreachableNodes,
		isolatedComponents: isolatedComponents,
		avgDegree:          avgDegree,
		maxDegree:          maxDegree,
		layerNodeCount:     layerNodeCount,
		maxLayer:           maxLayer,
		entrypointID:       entrypointID,
		entrypointDegree:   entrypointDegree,
		entrypointChanges:  entrypointChanges,
	}
}

func (m *Metrics) TombstoneReassignNeighbor() {
	if !m.enabled {
		return
	}

	m.tombstoneReassignNeighbors.Inc()
}

func (m *Metrics) TombstoneFindGlobalEntrypoint() {
	if !m.enabled {
		return
	}

	m.tombstoneFindGlobalEntrypoint.Inc()
}

func (m *Metrics) TombstoneFindLocalEntrypoint() {
	if !m.enabled {
		return
	}

	m.tombstoneFindLocalEntrypoint.Inc()
}

func (m *Metrics) SetTombstoneDeleteListSize(size int) {
	if !m.enabled {
		return
	}

	m.tombstoneDeleteListSize.Set(float64(size))
}

func (m *Metrics) AddTombstone() {
	if !m.enabled {
		return
	}

	m.tombstones.Inc()
}

func (m *Metrics) SetTombstone(count int) {
	if !m.enabled {
		return
	}

	m.tombstones.Set(float64(count))
}

func (m *Metrics) AddUnexpectedTombstone(operation string) {
	if !m.enabled {
		return
	}

	m.tombstoneUnexpected.With(prometheus.Labels{"operation": operation}).Inc()
}

func (m *Metrics) StartTombstoneCycle() {
	if !m.enabled {
		return
	}

	m.tombstoneStart.Set(float64(time.Now().Unix()))
	m.tombstoneProgress.Set(0)
	m.tombstoneEnd.Set(-1)
}

func (m *Metrics) EndTombstoneCycle() {
	if !m.enabled {
		return
	}

	m.tombstoneEnd.Set(float64(time.Now().Unix()))
}

func (m *Metrics) TombstoneCycleProgress(progress float64) {
	if !m.enabled {
		return
	}

	m.tombstoneProgress.Set(progress)
}

func (m *Metrics) RemoveTombstone() {
	if !m.enabled {
		return
	}

	m.tombstones.Dec()
}

func (m *Metrics) StartCleanup(threads int) {
	if !m.enabled {
		return
	}

	m.threads.Add(float64(threads))
}

func (m *Metrics) EndCleanup(threads int) {
	if !m.enabled {
		return
	}

	m.threads.Sub(float64(threads))
}

func (m *Metrics) CleanedUp() {
	if !m.enabled {
		return
	}

	m.cleaned.Inc()
}

func (m *Metrics) InsertVector() {
	if !m.enabled {
		return
	}

	m.insert.Inc()
}

func (m *Metrics) DeleteVector() {
	if !m.enabled {
		return
	}

	m.delete.Inc()
}

func (m *Metrics) SetSize(size int) {
	if !m.enabled {
		return
	}

	m.size.Set(float64(size))
}

func (m *Metrics) GrowDuration(start time.Time) {
	if !m.enabled {
		return
	}

	took := float64(time.Since(start)) / float64(time.Millisecond)
	m.grow.Observe(took)
}

type Observer func(start time.Time)

func noOpObserver(start time.Time) {
	// do nothing
}

func (m *Metrics) TrackInsertObserver(step string) Observer {
	if !m.enabled {
		return noOpObserver
	}

	curried := m.insertTime.With(prometheus.Labels{"step": step})

	return func(start time.Time) {
		took := float64(time.Since(start)) / float64(time.Millisecond)
		curried.Observe(took)
	}
}

func (m *Metrics) TrackDelete(start time.Time, step string) {
	if !m.enabled {
		return
	}

	took := float64(time.Since(start)) / float64(time.Millisecond)
	m.deleteTime.With(prometheus.Labels{"step": step}).Observe(took)
}

func (m *Metrics) StartupProgress(ratio float64) {
	if !m.enabled {
		return
	}

	m.startupProgress.Set(ratio)
}

func (m *Metrics) TrackStartupTotal(start time.Time) {
	if !m.enabled {
		return
	}

	took := float64(time.Since(start)) / float64(time.Millisecond)
	m.startupDurations.With(prometheus.Labels{"operation": "hnsw_read_all_commitlogs"}).Observe(took)
}

func (m *Metrics) TrackStartupIndividual(start time.Time) {
	if !m.enabled {
		return
	}

	took := float64(time.Since(start)) / float64(time.Millisecond)
	m.startupDurations.With(prometheus.Labels{"operation": "hnsw_read_single_commitlog"}).Observe(took)
}

func (m *Metrics) TrackStartupReadCommitlogDiskIO(read int64, nanoseconds int64) {
	if !m.enabled {
		return
	}

	seconds := float64(nanoseconds) / float64(time.Second)
	throughput := float64(read) / float64(seconds)
	m.startupDiskIO.With(prometheus.Labels{"operation": "hnsw_read_commitlog"}).Observe(throughput)
}

// HNSW Health Metrics (RFC 03: Enhanced Observability Suite)

func (m *Metrics) SetUnreachableNodes(count int) {
	if !m.enabled {
		return
	}
	m.unreachableNodes.Set(float64(count))
}

func (m *Metrics) SetIsolatedComponents(count int) {
	if !m.enabled {
		return
	}
	m.isolatedComponents.Set(float64(count))
}

func (m *Metrics) SetAvgDegree(layer int, avgDegree float64) {
	if !m.enabled {
		return
	}
	m.avgDegree.With(prometheus.Labels{"layer": fmt.Sprintf("%d", layer)}).Set(avgDegree)
}

func (m *Metrics) SetMaxDegree(layer int, maxDegree int) {
	if !m.enabled {
		return
	}
	m.maxDegree.With(prometheus.Labels{"layer": fmt.Sprintf("%d", layer)}).Set(float64(maxDegree))
}

func (m *Metrics) SetLayerNodeCount(layer int, count int) {
	if !m.enabled {
		return
	}
	m.layerNodeCount.With(prometheus.Labels{"layer": fmt.Sprintf("%d", layer)}).Set(float64(count))
}

func (m *Metrics) SetMaxLayer(layer int) {
	if !m.enabled {
		return
	}
	m.maxLayer.Set(float64(layer))
}

func (m *Metrics) SetEntrypointID(id uint64) {
	if !m.enabled {
		return
	}
	m.entrypointID.Set(float64(id))
}

func (m *Metrics) SetEntrypointDegree(degree int) {
	if !m.enabled {
		return
	}
	m.entrypointDegree.Set(float64(degree))
}

func (m *Metrics) IncrementEntrypointChanges() {
	if !m.enabled {
		return
	}
	m.entrypointChanges.Inc()
}

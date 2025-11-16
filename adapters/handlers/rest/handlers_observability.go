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

package rest

import (
	"github.com/go-openapi/runtime/middleware"
	"github.com/sirupsen/logrus"
	"github.com/weaviate/weaviate/adapters/handlers/rest/operations"
	"github.com/weaviate/weaviate/adapters/handlers/rest/operations/indices"
	"github.com/weaviate/weaviate/entities/models"
	"github.com/weaviate/weaviate/entities/observability"
	"github.com/weaviate/weaviate/usecases/auth/authorization/errors"
)

type indicesHealthHandlers struct {
	manager     indicesHealthManager
	logger      logrus.FieldLogger
	authorizer  authorizer
}

type indicesHealthManager interface {
	GetIndexHealth(className, shardName string) (*observability.IndexHealthResponse, error)
}

func (h *indicesHealthHandlers) getIndexHealth(params indices.GetIndicesClassNameShardNameHealthParams,
	principal *models.Principal,
) middleware.Responder {
	if err := h.authorizer.Authorize(principal, "get", "indices/*"); err != nil {
		return indices.NewGetIndicesClassNameShardNameHealthForbidden().
			WithPayload(errPayloadFromSingleErr(err))
	}

	health, err := h.manager.GetIndexHealth(params.ClassName, params.ShardName)
	if err != nil {
		h.logger.WithError(err).Error("failed to get index health")
		if errors.IsForbidden(err) {
			return indices.NewGetIndicesClassNameShardNameHealthForbidden().
				WithPayload(errPayloadFromSingleErr(err))
		}
		return indices.NewGetIndicesClassNameShardNameHealthInternalServerError().
			WithPayload(errPayloadFromSingleErr(err))
	}

	return indices.NewGetIndicesClassNameShardNameHealthOK().WithPayload(health)
}

func setupObservabilityHandlers(api *operations.WeaviateAPI,
	manager indicesHealthManager, logger logrus.FieldLogger, authorizer authorizer,
) {
	h := &indicesHealthHandlers{
		manager:    manager,
		logger:     logger,
		authorizer: authorizer,
	}

	// Note: This endpoint setup would need to be integrated with the OpenAPI spec
	// For now, this is a placeholder showing the intended structure
	// The actual wiring would happen in configure_api.go after updating the OpenAPI spec
	_ = h
}

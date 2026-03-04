package modifier

import (
	"context"

	"gpt-load/internal/models"
)

type ModificationContext struct {
	Context       context.Context
	OriginalGroup *models.Group
	SelectedGroup *models.Group
	OriginalModel string
	SelectedModel string
	IsAggregate   bool
	RequestPath   string
}

type RequestBodyModifier interface {
	Name() string
	Priority() int
	ShouldApply(ctx *ModificationContext) bool
	Modify(ctx *ModificationContext, bodyBytes []byte) (modifiedBytes []byte, modified bool, err error)
}

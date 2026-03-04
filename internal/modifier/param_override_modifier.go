package modifier

import (
	"gpt-load/internal/utils/jsonutil"
	"maps"

	"github.com/sirupsen/logrus"
)

type ParamOverrideModifier struct{}

func NewParamOverrideModifier() *ParamOverrideModifier {
	return &ParamOverrideModifier{}
}

func (m *ParamOverrideModifier) Name() string {
	return "ParamOverrideModifier"
}

func (m *ParamOverrideModifier) Priority() int {
	return 30
}

func (m *ParamOverrideModifier) ShouldApply(ctx *ModificationContext) bool {
	if ctx.IsAggregate && ctx.OriginalGroup.ID != ctx.SelectedGroup.ID {
		return len(ctx.OriginalGroup.ParamOverrides) > 0 || len(ctx.SelectedGroup.ParamOverrides) > 0
	}
	return len(ctx.SelectedGroup.ParamOverrides) > 0
}

func (m *ParamOverrideModifier) Modify(ctx *ModificationContext, bodyBytes []byte) ([]byte, bool, error) {
	var overrides map[string]any

	if ctx.IsAggregate && ctx.OriginalGroup.ID != ctx.SelectedGroup.ID {
		overrides = make(map[string]any)
		maps.Copy(overrides, ctx.OriginalGroup.ParamOverrides)
		maps.Copy(overrides, ctx.SelectedGroup.ParamOverrides)
	} else {
		overrides = ctx.SelectedGroup.ParamOverrides
	}

	if len(overrides) == 0 {
		return bodyBytes, false, nil
	}

	result, err := jsonutil.SetFields(bodyBytes, overrides)
	if err != nil {
		return nil, false, err
	}

	logrus.WithFields(logrus.Fields{
		"overrides": overrides,
	}).Debug("Param overrides applied")

	return result, true, nil
}

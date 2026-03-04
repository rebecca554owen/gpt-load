package modifier

import (
	"gpt-load/internal/utils/jsonutil"

	"github.com/sirupsen/logrus"
)

type ModelMappingModifier struct{}

func NewModelMappingModifier() *ModelMappingModifier {
	return &ModelMappingModifier{}
}

func (m *ModelMappingModifier) Name() string {
	return "ModelMappingModifier"
}

func (m *ModelMappingModifier) Priority() int {
	return 10
}

func (m *ModelMappingModifier) ShouldApply(ctx *ModificationContext) bool {
	return ctx.SelectedModel != "" && ctx.SelectedModel != ctx.OriginalModel
}

func (m *ModelMappingModifier) Modify(ctx *ModificationContext, bodyBytes []byte) ([]byte, bool, error) {
	result, err := jsonutil.SetField(bodyBytes, "model", ctx.SelectedModel)
	if err != nil {
		return nil, false, err
	}

	logrus.WithFields(logrus.Fields{
		"original_model": ctx.OriginalModel,
		"selected_model": ctx.SelectedModel,
	}).Debug("Model mapping applied")

	return result, true, nil
}

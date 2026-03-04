package modifier

import (
	"fmt"
	"gpt-load/internal/utils/jsonutil"

	"github.com/sirupsen/logrus"
)

type ModelRedirectModifier struct{}

func NewModelRedirectModifier() *ModelRedirectModifier {
	return &ModelRedirectModifier{}
}

func (m *ModelRedirectModifier) Name() string {
	return "ModelRedirectModifier"
}

func (m *ModelRedirectModifier) Priority() int {
	return 20
}

func (m *ModelRedirectModifier) ShouldApply(ctx *ModificationContext) bool {
	return len(ctx.SelectedGroup.ModelRedirectMap) > 0
}

func (m *ModelRedirectModifier) Modify(ctx *ModificationContext, bodyBytes []byte) ([]byte, bool, error) {
	currentModel, err := jsonutil.GetStringField(bodyBytes, "model")
	if err != nil {
		return nil, false, err
	}

	if actualModel, found := ctx.SelectedGroup.ModelRedirectMap[currentModel]; found {
		result, err := jsonutil.SetField(bodyBytes, "model", actualModel)
		if err != nil {
			return nil, false, err
		}

		logrus.WithFields(logrus.Fields{
			"original_model": currentModel,
			"redirected_to":  actualModel,
		}).Debug("Model redirect applied")

		return result, true, nil
	}

	if ctx.SelectedGroup.ModelRedirectStrict {
		return nil, false, fmt.Errorf("model '%s' is not configured in redirect rules", currentModel)
	}

	return bodyBytes, false, nil
}

package channel

import (
	"context"
	"gpt-load/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

func init() {
	Register("openai-response", newOpenAIResponseChannel)
}

type OpenAIResponseChannel struct {
	*OpenAIChannel
}

func newOpenAIResponseChannel(f *Factory, group *models.Group) (ChannelProxy, error) {
	base, err := f.newBaseChannel("openai-response", group)
	if err != nil {
		return nil, err
	}

	return &OpenAIResponseChannel{
		OpenAIChannel: &OpenAIChannel{
			BaseChannel: base,
		},
	}, nil
}

func (ch *OpenAIResponseChannel) ValidateKey(ctx context.Context, apiKey *models.APIKey, group *models.Group, model string) (bool, error) {
	return ch.validateKeyCommon(ctx, apiKey, group, model, ValidateKeyConfig{
		BuildPayload: func(testModel string) (map[string]any, error) {
			return gin.H{
				"model": testModel,
				"input": "hi",
			}, nil
		},
		SetAuthHeaders: func(req *http.Request, apiKey *models.APIKey) {
			req.Header.Set("Authorization", "Bearer "+apiKey.KeyValue)
		},
	})
}

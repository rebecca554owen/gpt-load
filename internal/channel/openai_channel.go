package channel

import (
	"context"
	"gpt-load/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

func init() {
	Register("openai", newOpenAIChannel)
}

type OpenAIChannel struct {
	*BaseChannel
}

func newOpenAIChannel(f *Factory, group *models.Group) (ChannelProxy, error) {
	base, err := f.newBaseChannel("openai", group)
	if err != nil {
		return nil, err
	}

	return &OpenAIChannel{
		BaseChannel: base,
	}, nil
}

// ModifyRequest sets the Authorization header for the OpenAI service.
func (ch *OpenAIChannel) ModifyRequest(req *http.Request, apiKey *models.APIKey, group *models.Group) {
	req.Header.Set("Authorization", "Bearer "+apiKey.KeyValue)
}

// ValidateKey checks if the given API key is valid by making a chat completion request.
func (ch *OpenAIChannel) ValidateKey(ctx context.Context, apiKey *models.APIKey, group *models.Group, model string) (bool, error) {
	return ch.validateKeyCommon(ctx, apiKey, group, model, ValidateKeyConfig{
		BuildPayload: func(testModel string) (map[string]any, error) {
			return gin.H{
				"model": testModel,
				"messages": []gin.H{
					{"role": "user", "content": "hi"},
				},
			}, nil
		},
		SetAuthHeaders: func(req *http.Request, apiKey *models.APIKey) {
			req.Header.Set("Authorization", "Bearer "+apiKey.KeyValue)
		},
	})
}

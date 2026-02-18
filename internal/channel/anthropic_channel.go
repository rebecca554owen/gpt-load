package channel

import (
	"context"
	"gpt-load/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

func init() {
	Register("anthropic", newAnthropicChannel)
}

type AnthropicChannel struct {
	*BaseChannel
}

func newAnthropicChannel(f *Factory, group *models.Group) (ChannelProxy, error) {
	base, err := f.newBaseChannel("anthropic", group)
	if err != nil {
		return nil, err
	}

	return &AnthropicChannel{
		BaseChannel: base,
	}, nil
}

// ModifyRequest sets the required headers for the Anthropic API.
func (ch *AnthropicChannel) ModifyRequest(req *http.Request, apiKey *models.APIKey, group *models.Group) {
	req.Header.Set("x-api-key", apiKey.KeyValue)
	req.Header.Set("anthropic-version", "2023-06-01")
}

// ValidateKey checks if the given API key is valid by making a messages request.
func (ch *AnthropicChannel) ValidateKey(ctx context.Context, apiKey *models.APIKey, group *models.Group, model string) (bool, error) {
	return ch.validateKeyCommon(ctx, apiKey, group, model, ValidateKeyConfig{
		BuildPayload: func(testModel string) (map[string]any, error) {
			return gin.H{
				"model":      testModel,
				"max_tokens": 100,
				"messages": []gin.H{
					{"role": "user", "content": "hi"},
				},
			}, nil
		},
		SetAuthHeaders: func(req *http.Request, apiKey *models.APIKey) {
			req.Header.Set("x-api-key", apiKey.KeyValue)
			req.Header.Set("anthropic-version", "2023-06-01")
		},
	})
}

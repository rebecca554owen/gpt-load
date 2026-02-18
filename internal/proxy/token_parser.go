package proxy

import (
	"encoding/json"
	"strings"
)

type TokenUsage struct {
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
	CachedTokens     int64
}

func (u *TokenUsage) Validate() {
	if u.CachedTokens > u.PromptTokens {
		u.CachedTokens = u.PromptTokens
	}
	u.TotalTokens = u.PromptTokens + u.CompletionTokens
}

func extractTokenFields(u map[string]interface{}, usage *TokenUsage) {
	if pt, ok := u["prompt_tokens"].(float64); ok {
		usage.PromptTokens = int64(pt)
	} else if it, ok := u["input_tokens"].(float64); ok {
		usage.PromptTokens = int64(it)
	}

	if ptd, ok := u["prompt_tokens_details"].(map[string]interface{}); ok {
		if ct, ok := ptd["cached_tokens"].(float64); ok {
			usage.CachedTokens = int64(ct)
		}
	}

	if ct, ok := u["cache_read_input_tokens"].(float64); ok {
		usage.CachedTokens = int64(ct)
	}

	if ct, ok := u["completion_tokens"].(float64); ok {
		usage.CompletionTokens = int64(ct)
	} else if ot, ok := u["output_tokens"].(float64); ok {
		usage.CompletionTokens = int64(ot)
	}
}

func EstimatePromptTokensFromRequest(body []byte) *TokenUsage {
	if len(body) == 0 {
		return nil
	}

	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil
	}

	totalChars := 0

	if messages, ok := req["messages"].([]interface{}); ok {
		for _, m := range messages {
			if msg, ok := m.(map[string]interface{}); ok {
				if content, ok := msg["content"]; ok {
					totalChars += estimateContentLength(content)
				}
			}
		}
	}

	if system, ok := req["system"].(string); ok {
		totalChars += len(system)
	}
	if prompt, ok := req["prompt"].(string); ok {
		totalChars += len(prompt)
	}

	if input, ok := req["input"]; ok {
		totalChars += estimateContentLength(input)
	}
	if instructions, ok := req["instructions"].(string); ok {
		totalChars += len(instructions)
	}

	if totalChars == 0 {
		return nil
	}

	estimated := int64(float64(totalChars) / 2.5)
	if estimated < 1 {
		estimated = 1
	}

	return &TokenUsage{
		PromptTokens: estimated,
		TotalTokens:  estimated,
	}
}

func estimateContentLength(content interface{}) int {
	if s, ok := content.(string); ok {
		return len(s)
	}

	if arr, ok := content.([]interface{}); ok {
		total := 0
		for _, item := range arr {
			if obj, ok := item.(map[string]interface{}); ok {
				if t, ok := obj["type"].(string); ok {
					switch t {
					case "text":
						if text, ok := obj["text"].(string); ok {
							total += len(text)
						}
					case "image", "image_url", "image-url":
						total += 400
					default:
						if text, ok := obj["text"].(string); ok {
							total += len(text)
						}
					}
				}
			}
		}
		return total
	}

	return 0
}

func ParseUsage(body []byte) *TokenUsage {
	if len(body) == 0 {
		return nil
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}

	usage := &TokenUsage{}

	if it, ok := resp["input_tokens"].(float64); ok {
		usage.PromptTokens = int64(it)
		usage.Validate()
		return usage
	}

	u, ok := resp["usage"].(map[string]interface{})
	if !ok {
		return nil
	}

	extractTokenFields(u, usage)

	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		return nil
	}

	usage.Validate()
	return usage
}

func ParseUsageFromStream(tailData []byte) *TokenUsage {
	if len(tailData) == 0 || !strings.Contains(string(tailData), `"usage"`) {
		return nil
	}

	var lastUsage *TokenUsage
	lines := strings.Split(string(tailData), "\n")

	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || line == "data: [DONE]" {
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			line = strings.TrimPrefix(line, "data: ")
		}

		if !strings.Contains(line, `"usage"`) {
			continue
		}

		var data map[string]interface{}
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			continue
		}

		u, ok := data["usage"].(map[string]interface{})
		if !ok {
			continue
		}

		usage := &TokenUsage{}
		extractTokenFields(u, usage)

		if usage.PromptTokens > 0 || usage.CompletionTokens > 0 {
			usage.Validate()
			lastUsage = usage
			break
		}
	}

	return lastUsage
}

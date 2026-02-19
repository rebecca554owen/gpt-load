package proxy

import (
	"encoding/json"
	"strings"
)

type TokenUsage struct {
	PromptTokens     int64
	CompletionTokens int64
	CachedTokens     int64
}

func (u *TokenUsage) Total() int64 {
	return u.PromptTokens + u.CompletionTokens
}

func extractTokenFields(u map[string]interface{}, usage *TokenUsage) {
	if pt, ok := u["prompt_tokens"].(float64); ok {
		usage.PromptTokens = int64(pt)
	} else if it, ok := u["input_tokens"].(float64); ok {
		usage.PromptTokens = int64(it)
	}

	if ct, ok := u["completion_tokens"].(float64); ok {
		usage.CompletionTokens = int64(ct)
	} else if ot, ok := u["output_tokens"].(float64); ok {
		usage.CompletionTokens = int64(ot)
	}

	if usage.CachedTokens == 0 {
		if ptd, ok := u["prompt_tokens_details"].(map[string]interface{}); ok {
			if ct, ok := ptd["cached_tokens"].(float64); ok {
				usage.CachedTokens = int64(ct)
			}
		}
	}

	if usage.CachedTokens == 0 {
		if itd, ok := u["input_tokens_details"].(map[string]interface{}); ok {
			if ct, ok := itd["cached_tokens"].(float64); ok {
				usage.CachedTokens = int64(ct)
			}
		}
	}

	if usage.CachedTokens == 0 {
		if ct, ok := u["cache_read_input_tokens"].(float64); ok {
			usage.CachedTokens = int64(ct)
		}
	}
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

	if u, ok := resp["usage"].(map[string]interface{}); ok {
		extractTokenFields(u, usage)
	} else {
		if it, ok := resp["input_tokens"].(float64); ok {
			usage.PromptTokens = int64(it)
		}
		if ot, ok := resp["output_tokens"].(float64); ok {
			usage.CompletionTokens = int64(ot)
		}
		if ct, ok := resp["cache_read_input_tokens"].(float64); ok {
			usage.CachedTokens = int64(ct)
		}
	}

	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		return nil
	}

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

		usage := &TokenUsage{}
		found := false

		if u, ok := data["usage"].(map[string]interface{}); ok {
			extractTokenFields(u, usage)
			found = true
		}

		if msg, ok := data["message"].(map[string]interface{}); ok {
			if u, ok := msg["usage"].(map[string]interface{}); ok {
				extractTokenFields(u, usage)
				found = true
			}
		}

		if found && (usage.PromptTokens > 0 || usage.CompletionTokens > 0) {
			lastUsage = usage
			break
		}
	}

	return lastUsage
}

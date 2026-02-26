package proxy

import (
	"encoding/json"
	"math"
	"strings"
	"sync"
	"unicode"

	"github.com/sirupsen/logrus"
)

type Provider string

const (
	OpenAI  Provider = "openai"
	Gemini  Provider = "gemini"
	Claude  Provider = "claude"
	Unknown Provider = "unknown"
)

type multipliers struct {
	Word       float64
	Number     float64
	CJK        float64
	Symbol     float64
	MathSymbol float64
	URLDelim   float64
	AtSign     float64
	Emoji      float64
	Newline    float64
	Space      float64
	BasePad    int
}

var (
	multipliersMap = map[Provider]multipliers{
		Gemini: {
			Word: 1.15, Number: 2.8, CJK: 0.68, Symbol: 0.38, MathSymbol: 1.05, URLDelim: 1.2, AtSign: 2.5, Emoji: 1.08, Newline: 1.15, Space: 0.2, BasePad: 0,
		},
		Claude: {
			Word: 1.13, Number: 1.63, CJK: 1.21, Symbol: 0.4, MathSymbol: 4.52, URLDelim: 1.26, AtSign: 2.82, Emoji: 2.6, Newline: 0.89, Space: 0.39, BasePad: 0,
		},
		OpenAI: {
			Word: 1.02, Number: 1.55, CJK: 0.85, Symbol: 0.4, MathSymbol: 2.68, URLDelim: 1.0, AtSign: 2.0, Emoji: 2.12, Newline: 0.5, Space: 0.42, BasePad: 0,
		},
	}
	multipliersLock sync.RWMutex
)

func getMultipliers(p Provider) multipliers {
	multipliersLock.RLock()
	defer multipliersLock.RUnlock()

	switch p {
	case Gemini:
		return multipliersMap[Gemini]
	case Claude:
		return multipliersMap[Claude]
	case OpenAI:
		return multipliersMap[OpenAI]
	default:
		return multipliersMap[OpenAI]
	}
}

func EstimateToken(provider Provider, text string) int {
	m := getMultipliers(provider)
	var count float64

	type WordType int
	const (
		None WordType = iota
		Latin
		Number
	)
	currentWordType := None

	for _, r := range text {
		if unicode.IsSpace(r) {
			currentWordType = None
			if r == '\n' || r == '\t' {
				count += m.Newline
			} else {
				count += m.Space
			}
			continue
		}

		if isCJK(r) {
			currentWordType = None
			count += m.CJK
			continue
		}

		if isEmoji(r) {
			currentWordType = None
			count += m.Emoji
			continue
		}

		if isLatinOrNumber(r) {
			isNum := unicode.IsNumber(r)
			newType := Latin
			if isNum {
				newType = Number
			}

			if currentWordType == None || currentWordType != newType {
				if newType == Number {
					count += m.Number
				} else {
					count += m.Word
				}
				currentWordType = newType
			}
			continue
		}

		currentWordType = None
		if isMathSymbol(r) {
			count += m.MathSymbol
		} else if r == '@' {
			count += m.AtSign
		} else if isURLDelim(r) {
			count += m.URLDelim
		} else {
			count += m.Symbol
		}
	}

	return int(math.Ceil(count)) + m.BasePad
}

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		(r >= 0x3040 && r <= 0x30FF) ||
		(r >= 0xAC00 && r <= 0xD7A3)
}

func isLatinOrNumber(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r)
}

func isEmoji(r rune) bool {
	return (r >= 0x1F300 && r <= 0x1F9FF) ||
		(r >= 0x2600 && r <= 0x26FF) ||
		(r >= 0x2700 && r <= 0x27BF) ||
		(r >= 0x1F600 && r <= 0x1F64F) ||
		(r >= 0x1F900 && r <= 0x1F9FF) ||
		(r >= 0x1FA00 && r <= 0x1FAFF)
}

var (
	mathSymbolMap map[rune]bool
	mathSymbolRanges []struct{ min, max rune }
	urlDelimMap map[rune]bool
)

func init() {
	mathSymbolMap = make(map[rune]bool)
	mathSymbols := "∑∫∂√∞≤≥≠≈±×÷∈∉∋∌⊂⊃⊆⊇∪∩∧∨¬∀∃∄∅∆∇∝∟∠∡∢°′″‴⁺⁻⁼⁽⁾ⁿ₀₁₂₃₄₅₆₇₈₉₊₋₌₍₎²³¹⁴⁵⁶⁷⁸⁹⁰"
	for _, m := range mathSymbols {
		mathSymbolMap[m] = true
	}
	mathSymbolRanges = []struct{ min, max rune }{
		{0x2200, 0x22FF},
		{0x2A00, 0x2AFF},
		{0x1D400, 0x1D7FF},
	}

	urlDelimMap = make(map[rune]bool)
	urlDelims := "/:?&=;#%"
	for _, d := range urlDelims {
		urlDelimMap[d] = true
	}
}

func isMathSymbol(r rune) bool {
	if mathSymbolMap[r] {
		return true
	}
	for _, rng := range mathSymbolRanges {
		if r >= rng.min && r <= rng.max {
			return true
		}
	}
	return false
}

func isURLDelim(r rune) bool {
	return urlDelimMap[r]
}

func EstimateTokenByModel(model, text string) int {
	if text == "" {
		return 0
	}

	model = strings.ToLower(model)
	if strings.Contains(model, "gemini") {
		return EstimateToken(Gemini, text)
	} else if strings.Contains(model, "claude") {
		return EstimateToken(Claude, text)
	} else {
		return EstimateToken(OpenAI, text)
	}
}

type TokenUsage struct {
	PromptTokens     int64
	CompletionTokens int64
	CachedTokens     int64
}

func (u *TokenUsage) Total() int64 {
	return u.PromptTokens + u.CompletionTokens
}

func extractTokenFields(u map[string]interface{}, usage *TokenUsage, channelType string) {
	// Extract prompt/input tokens
	if pt, ok := u["prompt_tokens"].(float64); ok {
		usage.PromptTokens = int64(pt)
	} else if it, ok := u["input_tokens"].(float64); ok {
		usage.PromptTokens = int64(it)
	}

	// Extract completion/output tokens
	if ct, ok := u["completion_tokens"].(float64); ok {
		usage.CompletionTokens = int64(ct)
	} else if ot, ok := u["output_tokens"].(float64); ok {
		usage.CompletionTokens = int64(ot)
	}

	// Extract cached tokens from various locations
	extractCachedTokens(u, usage)

	// Process cache tokens based on channel type
	processCacheByChannel(usage, channelType)
}

func extractCachedTokens(u map[string]interface{}, usage *TokenUsage) {
	// Try OpenAI prompt_tokens_details.cached_tokens
	if usage.CachedTokens == 0 {
		if ptd, ok := u["prompt_tokens_details"].(map[string]interface{}); ok {
			if ct, ok := ptd["cached_tokens"].(float64); ok {
				usage.CachedTokens = int64(ct)
			}
		}
	}

	// Try OpenAI compatible input_tokens_details.cached_tokens
	if usage.CachedTokens == 0 {
		if itd, ok := u["input_tokens_details"].(map[string]interface{}); ok {
			if ct, ok := itd["cached_tokens"].(float64); ok {
				usage.CachedTokens = int64(ct)
			}
		}
	}

	// Try Anthropic cache_read_input_tokens
	if usage.CachedTokens == 0 {
		if ct, ok := u["cache_read_input_tokens"].(float64); ok {
			usage.CachedTokens = int64(ct)
		}
	}
}

func processCacheByChannel(usage *TokenUsage, channelType string) {
	switch channelType {
	case "anthropic":
		// Claude's input_tokens does not include cache_read_input_tokens
		// We need to add them to get the total prompt tokens
		if usage.CachedTokens > 0 {
			usage.PromptTokens += usage.CachedTokens
		}

	default:
		// OpenAI and other channels: prompt_tokens already includes cached_tokens
		// Validate that cached_tokens does not exceed prompt_tokens
		if usage.CachedTokens > usage.PromptTokens && usage.PromptTokens > 0 {
			logrus.WithFields(logrus.Fields{
				"cached_tokens": usage.CachedTokens,
				"prompt_tokens": usage.PromptTokens,
				"channel":       channelType,
			}).Warn("Cache tokens exceed prompt tokens, capping to prompt tokens")
			usage.CachedTokens = usage.PromptTokens
		}
	}
}

func ParseUsage(body []byte, channelType string) *TokenUsage {
	if len(body) == 0 {
		return nil
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}

	usage := &TokenUsage{}

	if u, ok := resp["usage"].(map[string]interface{}); ok {
		extractTokenFields(u, usage, channelType)
	} else {
		// Try Anthropic format (top-level fields)
		if it, ok := resp["input_tokens"].(float64); ok {
			usage.PromptTokens = int64(it)
		}
		if ot, ok := resp["output_tokens"].(float64); ok {
			usage.CompletionTokens = int64(ot)
		}
		if ct, ok := resp["cache_read_input_tokens"].(float64); ok {
			usage.CachedTokens = int64(ct)
		}
		// Process cache for Anthropic format
		processCacheByChannel(usage, channelType)
	}

	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		return nil
	}

	return usage
}

func ParseUsageFromStream(tailData []byte, channelType string) *TokenUsage {
	if len(tailData) == 0 {
		return nil
	}

	if !strings.Contains(string(tailData), `"usage"`) {
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
			extractTokenFields(u, usage, channelType)
			found = true
		}

		if msg, ok := data["message"].(map[string]interface{}); ok {
			if u, ok := msg["usage"].(map[string]interface{}); ok {
				extractTokenFields(u, usage, channelType)
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

// estimateTokens estimates token usage from request and response bodies
func estimateTokens(model string, requestBody, responseBody []byte) *TokenUsage {
	if len(requestBody) == 0 || len(responseBody) == 0 {
		return nil
	}

	usage := &TokenUsage{}

	// Extract prompt text from request messages
	promptText := extractPromptFromRequest(requestBody)
	if promptText != "" {
		usage.PromptTokens = int64(EstimateTokenByModel(model, promptText))
	}

	// Extract completion text from response
	completionText := extractCompletionFromResponse(responseBody)
	if completionText != "" {
		usage.CompletionTokens = int64(EstimateTokenByModel(model, completionText))
	}

	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		return nil
	}

	return usage
}

// estimateTokensFromStream estimates token usage from streaming response
func estimateTokensFromStream(model string, requestBody, streamData []byte) *TokenUsage {
	if len(requestBody) == 0 || len(streamData) == 0 {
		return nil
	}

	usage := &TokenUsage{}

	// Extract prompt text from request messages
	promptText := extractPromptFromRequest(requestBody)
	if promptText != "" {
		usage.PromptTokens = int64(EstimateTokenByModel(model, promptText))
	}

	// Extract completion text from streaming SSE data
	completionText := extractCompletionFromStream(streamData)
	if completionText != "" {
		usage.CompletionTokens = int64(EstimateTokenByModel(model, completionText))
	}

	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		return nil
	}

	return usage
}

// extractPromptFromRequest extracts prompt text from request body
func extractPromptFromRequest(body []byte) string {
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}

	var textBuilder strings.Builder

	// Extract from messages array (OpenAI/Claude format)
	if messages, ok := req["messages"].([]interface{}); ok {
		for _, msg := range messages {
			if m, ok := msg.(map[string]interface{}); ok {
				// Handle content as string
				if content, ok := m["content"].(string); ok {
					textBuilder.WriteString(content)
					textBuilder.WriteString(" ")
				}
				// Handle content as array (multimodal)
				if contentArr, ok := m["content"].([]interface{}); ok {
					for _, item := range contentArr {
						if itemMap, ok := item.(map[string]interface{}); ok {
							if text, ok := itemMap["text"].(string); ok {
								textBuilder.WriteString(text)
								textBuilder.WriteString(" ")
							}
						}
					}
				}
			}
		}
	}

	// Extract from prompt field (completion API)
	if prompt, ok := req["prompt"].(string); ok {
		textBuilder.WriteString(prompt)
	}

	return textBuilder.String()
}

// extractCompletionFromResponse extracts completion text from response body
func extractCompletionFromResponse(body []byte) string {
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return ""
	}

	var textBuilder strings.Builder

	// OpenAI format: choices[].message.content
	if choices, ok := resp["choices"].([]interface{}); ok {
		for _, choice := range choices {
			if c, ok := choice.(map[string]interface{}); ok {
				if message, ok := c["message"].(map[string]interface{}); ok {
					if content, ok := message["content"].(string); ok {
						textBuilder.WriteString(content)
						textBuilder.WriteString(" ")
					}
				}
				// Handle text field (completion API)
				if text, ok := c["text"].(string); ok {
					textBuilder.WriteString(text)
					textBuilder.WriteString(" ")
				}
			}
		}
	}

	// Claude format: content[].text
	if content, ok := resp["content"].([]interface{}); ok {
		for _, item := range content {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if text, ok := itemMap["text"].(string); ok {
					textBuilder.WriteString(text)
					textBuilder.WriteString(" ")
				}
			}
		}
	}

	return textBuilder.String()
}

// parseStreamChunks parses SSE data chunks and calls the provided function for each chunk
func parseStreamChunks(data []byte, fn func(chunk map[string]interface{})) {
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "data: [DONE]" || line == "[DONE]" {
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			line = strings.TrimPrefix(line, "data: ")
		}

		if line == "" {
			continue
		}

		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}

		fn(chunk)
	}
}

// extractCompletionFromStream extracts completion text from SSE streaming data
func extractCompletionFromStream(streamData []byte) string {
	var textBuilder strings.Builder
	parseStreamChunks(streamData, func(data map[string]interface{}) {
		// OpenAI format: choices[].delta.content
		if choices, ok := data["choices"].([]interface{}); ok {
			for _, choice := range choices {
				if c, ok := choice.(map[string]interface{}); ok {
					if delta, ok := c["delta"].(map[string]interface{}); ok {
						if content, ok := delta["content"].(string); ok {
							textBuilder.WriteString(content)
						}
					}
				}
			}
		}
		// Claude format: delta.text
		if delta, ok := data["delta"].(map[string]interface{}); ok {
			if text, ok := delta["text"].(string); ok {
				textBuilder.WriteString(text)
			}
		}
	})
	return textBuilder.String()
}

// containsContent checks if the chunk contains actual token content (not just control messages)
func containsContent(chunk []byte) bool {
	found := false
	parseStreamChunks(chunk, func(data map[string]interface{}) {
		// OpenAI format: choices[].delta.content
		if choices, ok := data["choices"].([]interface{}); ok {
			for _, choice := range choices {
				if c, ok := choice.(map[string]interface{}); ok {
					if delta, ok := c["delta"].(map[string]interface{}); ok {
						if content, ok := delta["content"].(string); ok && content != "" {
							found = true
							return
						}
					}
				}
			}
		}
		// Claude format: delta.text
		if delta, ok := data["delta"].(map[string]interface{}); ok {
			if text, ok := delta["text"].(string); ok && text != "" {
				found = true
			}
		}
	})
	return found
}

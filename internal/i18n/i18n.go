package i18n

import (
	"encoding/json"
	"fmt"
	"strings"

	"gpt-load/internal/i18n/locales"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var (
	bundle *i18n.Bundle
)

// Init initializes i18n
func Init() error {
	bundle = i18n.NewBundle(language.Chinese)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	// Load supported language files
	languages := []string{"zh-CN", "en-US", "ja-JP"}
	for _, lang := range languages {
		if err := loadMessageFile(lang); err != nil {
			return fmt.Errorf("failed to load language file %s: %w", lang, err)
		}
	}

	return nil
}

// loadMessageFile loads language file
func loadMessageFile(lang string) error {
	// Set messages based on language
	messages := getMessages(lang)
	for id, msg := range messages {
		bundle.AddMessages(language.MustParse(lang), &i18n.Message{
			ID:    id,
			Other: msg,
		})
	}

	return nil
}

// GetLocalizer gets localizer
func GetLocalizer(acceptLang string) *i18n.Localizer {
	// Parse Accept-Language header
	langs := parseAcceptLanguage(acceptLang)

	// Default to Chinese if no language specified
	if len(langs) == 0 {
		langs = []string{"zh-CN"}
	}

	return i18n.NewLocalizer(bundle, langs...)
}

// parseAcceptLanguage parses Accept-Language header
func parseAcceptLanguage(acceptLang string) []string {
	if acceptLang == "" {
		return nil
	}

	// Simple parsing, only take the first language
	parts := strings.Split(acceptLang, ",")
	if len(parts) > 0 {
		lang := strings.TrimSpace(parts[0])
		// Remove quality factor (q=...)
		if idx := strings.Index(lang, ";"); idx > 0 {
			lang = lang[:idx]
		}

		// Normalize language code
		lang = normalizeLanguageCode(lang)
		return []string{lang}
	}

	return nil
}

// normalizeLanguageCode normalizes language code
func normalizeLanguageCode(lang string) string {
	lang = strings.TrimSpace(lang)

	// Map common language codes
	switch strings.ToLower(lang) {
	case "zh", "zh-cn", "zh-hans":
		return "zh-CN"
	case "en", "en-us":
		return "en-US"
	case "ja", "ja-jp":
		return "ja-JP"
	default:
		// Try to match prefix
		if strings.HasPrefix(strings.ToLower(lang), "zh") {
			return "zh-CN"
		}
		if strings.HasPrefix(strings.ToLower(lang), "en") {
			return "en-US"
		}
		if strings.HasPrefix(strings.ToLower(lang), "ja") {
			return "ja-JP"
		}
		// Default to Chinese
		return "zh-CN"
	}
}

// T translates message
func T(localizer *i18n.Localizer, msgID string, data ...map[string]any) string {
	config := &i18n.LocalizeConfig{
		MessageID: msgID,
	}

	if len(data) > 0 {
		config.TemplateData = data[0]
	}

	msg, err := localizer.Localize(config)
	if err != nil {
		// Return message ID if translation fails
		return msgID
	}

	return msg
}

// getMessages gets language messages
func getMessages(lang string) map[string]string {
	switch lang {
	case "en-US":
		return locales.MessagesEnUS
	case "ja-JP":
		return locales.MessagesJaJP
	default:
		return locales.MessagesZhCN
	}
}

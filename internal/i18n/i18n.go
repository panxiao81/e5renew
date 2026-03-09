package i18n

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localesFS embed.FS

type Bundle struct {
	*i18n.Bundle
}

type Localizer struct {
	*i18n.Localizer
}

var DefaultBundle *Bundle

func Init() error {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	// Load all translation files
	files, err := localesFS.ReadDir("locales")
	if err != nil {
		return fmt.Errorf("failed to read locales directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		path := "locales/" + file.Name()
		_, err := bundle.LoadMessageFileFS(localesFS, path)
		if err != nil {
			return fmt.Errorf("failed to load translation file %s: %w", path, err)
		}
	}

	DefaultBundle = &Bundle{Bundle: bundle}
	return nil
}

func (b *Bundle) GetLocalizer(langs ...string) *Localizer {
	return &Localizer{
		Localizer: i18n.NewLocalizer(b.Bundle, langs...),
	}
}

func (l *Localizer) T(messageID string, templateData map[string]interface{}) string {
	message, err := l.Localize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: templateData,
	})
	if err != nil {
		return messageID // Fallback to message ID if translation not found
	}
	return message
}

func (l *Localizer) TDefault(messageID, defaultMessage string, templateData map[string]interface{}) string {
	message, err := l.Localize(&i18n.LocalizeConfig{
		MessageID:      messageID,
		DefaultMessage: &i18n.Message{ID: messageID, Other: defaultMessage},
		TemplateData:   templateData,
	})
	if err != nil {
		return defaultMessage
	}
	return message
}

// GetLanguageFromRequest extracts the preferred language from the HTTP request
func GetLanguageFromRequest(r *http.Request) string {
	// Check URL parameter first
	if lang := r.URL.Query().Get("lang"); lang != "" {
		return lang
	}

	// Check cookie
	if cookie, err := r.Cookie("lang"); err == nil {
		return cookie.Value
	}

	// Check Accept-Language header
	acceptLang := r.Header.Get("Accept-Language")
	if acceptLang != "" {
		tags, _, _ := language.ParseAcceptLanguage(acceptLang)
		if len(tags) > 0 {
			return tags[0].String()
		}
	}

	return "en" // Default to English
}

// ContextKey for storing localizer in context
type contextKey string

const LocalizerKey contextKey = "localizer"

// WithLocalizer adds a localizer to the context
func WithLocalizer(ctx context.Context, localizer *Localizer) context.Context {
	return context.WithValue(ctx, LocalizerKey, localizer)
}

// FromContext retrieves a localizer from the context
func FromContext(ctx context.Context) *Localizer {
	if localizer, ok := ctx.Value(LocalizerKey).(*Localizer); ok {
		return localizer
	}
	// Return default English localizer if none found
	return DefaultBundle.GetLocalizer("en")
}

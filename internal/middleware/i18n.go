package middleware

import (
	"net/http"
	"time"

	"github.com/panxiao81/e5renew/internal/cookiepolicy"
	"github.com/panxiao81/e5renew/internal/i18n"
)

// I18nMiddleware handles language detection and sets up localizer in context
func I18nMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle language switching
		if lang := r.URL.Query().Get("lang"); lang != "" {
			// Set language cookie for persistence
			cookie := &http.Cookie{
				Name:     "lang",
				Value:    lang,
				Path:     "/",
				MaxAge:   int((365 * 24 * time.Hour).Seconds()), // 1 year
				Secure:   cookiepolicy.RequestUsesHTTPS(r),
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			}
			http.SetCookie(w, cookie)
		}

		// Get preferred language
		lang := i18n.GetLanguageFromRequest(r)

		// Create localizer and add to context
		localizer := i18n.DefaultBundle.GetLocalizer(lang)
		ctx := i18n.WithLocalizer(r.Context(), localizer)

		// Continue with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

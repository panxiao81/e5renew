package environment

import (
	"log/slog"

	"github.com/alexedwards/scs/v2"
	"github.com/panxiao81/e5renew/internal/db"
	"github.com/panxiao81/e5renew/internal/view"
)

type Application struct {
	Logger         *slog.Logger
	Template       *view.Template
	SessionManager *scs.SessionManager
	DB             db.Database
}

func NewApplication(logger *slog.Logger, template *view.Template, scs *scs.SessionManager, db db.Database) *Application {
	return &Application{
		Logger:         logger,
		Template:       template,
		SessionManager: scs,
		DB:             db,
	}
}

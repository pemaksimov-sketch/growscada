package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/google/uuid"
	"github.com/kipitix/gracedown"
	"github.com/kipitix/growscada/internal/server/application"
	"github.com/kipitix/growscada/internal/server/domain/event"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/project"
	"github.com/kipitix/growscada/internal/server/infrastructure/postgres/outbox"
	"github.com/kipitix/growscada/internal/server/infrastructure/postgres/repositories"
	"github.com/kipitix/growscada/internal/server/interface/eventbus"
	"github.com/kipitix/growscada/internal/server/interface/restapi"
	"github.com/kipitix/growscada/internal/server/interface/ui/root"
	"github.com/kipitix/growscada/internal/server/interface/ui/uiutil"
	"github.com/maxence-charriere/go-app/v10/pkg/app"

	_ "github.com/lib/pq"
)

const (
	apiServerURL = "http://localhost:9090"
)

// serverArgs is the server's command-line/environment configuration.
type serverArgs struct {
	DatabaseDSN   string `arg:"env:DATABASE_DSN" default:"postgres://growscada:growscada@localhost:5432/growscada?sslmode=disable" help:"PostgreSQL connection string"`
	MaxSSEClients int    `arg:"env:MAX_SSE_CLIENTS" default:"100" help:"maximum number of concurrent SSE event-stream connections"`
}

type slogAdapter struct{}

func (slogAdapter) Println(v ...any) {
	slog.Info(fmt.Sprint(v...))
}

func main() {
	// Server setup for serving the client-side app (PWA)
	app.Route("/", func() app.Composer {
		r := root.NewRoot(apiServerURL)
		return r
	})

	if app.IsClient {
		uiutil.SendSchemaVersion()
	}

	// Required go-app framework call to initialize the PWA
	app.RunWhenOnBrowser()

	var cliArgs serverArgs
	arg.MustParse(&cliArgs)

	// Create a graceful shutdown manager
	gracedownManager := gracedown.NewManager(gracedown.WithLogger(slogAdapter{}))

	// INFRASTRUCTURE COMPONENTS

	// Create database connection
	sqlDB, err := sql.Open("postgres", cliArgs.DatabaseDSN)
	// Add hook to shutdown database connection
	gracedownManager.RegisterInfrastructure("Database", 15*time.Second, func(ctx context.Context) error {
		if sqlDB != nil {
			return sqlDB.Close()
		}
		return nil
	})
	// Check DB open error
	if err != nil {
		slog.Error("Database connection error", "error", err)
		emergencyExit(gracedownManager, gracedown.ExitIOErr)
	}
	// Check DB connection
	err = sqlDB.Ping()
	if err != nil {
		slog.Error("Database ping error", "error", err)
		emergencyExit(gracedownManager, gracedown.ExitIOErr)
	}

	// Create message broker connection
	// TODO
	// Add hook to shutdown message broker connection
	// Create event bus: the SSE stream's source of events
	eventBus := eventbus.NewEventBus()
	// Create the outbox dispatcher: it delivers the events of committed
	// changes to the event bus (ADR 0008)
	dispatcher := outbox.NewDispatcher(sqlDB, eventBus.Publish)
	dispatcherCtx, stopDispatcher := context.WithCancel(context.Background())
	dispatcherDone := make(chan struct{})
	// stopOutbox stops the dispatcher and delivers what is left in the outbox
	stopOutbox := func(ctx context.Context) {
		stopDispatcher()
		<-dispatcherDone
		if _, err := dispatcher.DrainOnce(ctx); err != nil {
			slog.Error("Outbox not drained on shutdown", "error", err)
		}
	}
	notifyDispatcher := repositories.NotifyOnCommit(dispatcher.Notify)

	// INTERFACE COMPONENTS
	// API server
	projectRepository := repositories.NewProjectRepositoryPostgres(sqlDB, project.DefaultOrganizationID(), notifyDispatcher)
	projectService := application.NewProjectService(projectRepository, project.DefaultOrganizationID())
	mainRepositories := repositories.ForProject(sqlDB, project.MainID(), notifyDispatcher)
	tagRepository := mainRepositories.Tags()
	widgetTypeRepository := repositories.NewWidgetTypeRepositoryPostgres(sqlDB, notifyDispatcher)
	sceneRepository := mainRepositories.Scenes()
	bindingScopes := func(ctx context.Context) ([]application.WidgetBindingScope, error) {
		projects, err := projectRepository.FindAll(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]application.WidgetBindingScope, len(projects))
		for i, p := range projects {
			scope := repositories.ForProject(sqlDB, p.ID(), notifyDispatcher)
			result[i] = application.WidgetBindingScope{Scenes: scope.Scenes(), Tags: scope.Tags()}
		}
		return result, nil
	}
	reconciliation := application.NewProjectWidgetReconciliationPolicy(widgetTypeRepository, bindingScopes)
	dispatcher.RegisterPolicy("reconcile-widget-bindings", event.EventTypeWidgetTypeUpdated, reconciliation.Handle)
	go func() {
		dispatcher.Run(dispatcherCtx)
		close(dispatcherDone)
	}()
	// Create services
	tagService := application.NewTagService(tagRepository)
	widgetTypeService := application.NewWidgetTypeService(widgetTypeRepository, sceneRepository, tagRepository, application.WithWidgetTypeScopes(bindingScopes))
	sceneService := application.NewSceneService(sceneRepository, widgetTypeRepository, tagRepository)
	// Create router
	apiRouter := restapi.NewRouter(tagService, widgetTypeService, sceneService, eventBus, cliArgs.MaxSSEClients,
		restapi.WithProjects(projectService, func(rawID uuid.UUID) restapi.ProjectServices {
			scope := repositories.ForProject(sqlDB, id.NewID(id.IDWithUUID[project.Project](rawID)), notifyDispatcher)
			tags, scenes := scope.Tags(), scope.Scenes()
			return restapi.ProjectServices{Tags: application.NewTagService(tags), Scenes: application.NewSceneService(scenes, widgetTypeRepository, tags)}
		}))
	// Register the event hub shutdown handler. The outbox is drained first,
	// while SSE clients are still connected, so what is left in it reaches
	// them; a change committed after the drain (the API server stops
	// concurrently) stays in the outbox until the next start. The database
	// closes later, in the infrastructure layer.
	gracedownManager.RegisterInterface("Event Hub", 15*time.Second, func(ctx context.Context) error {
		stopOutbox(ctx)
		apiRouter.Close()
		return nil
	})
	// Start HTTP server for API
	apiServer := &http.Server{
		Addr:    ":9090",
		Handler: apiRouter.ServeMux(),
	}

	// Register the API server shutdown handler
	gracedownManager.RegisterInterface("API HTTP Server", 15*time.Second, func(ctx context.Context) error {
		return apiServer.Shutdown(ctx)
	})

	// Start the API server in a separate goroutine
	go func() {
		slog.Info("API Server starting on :9090")
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("API Server error", "error", err)
		}
	}()

	// Server for serving the PWA client
	pwaServer := &http.Server{
		Addr: ":8080",
		Handler: &app.Handler{
			Name:        "GrowSCADA",
			Description: "SCADA to Go",
		},
	}
	// Register the PWA server shutdown handler
	gracedownManager.RegisterInterface("PWA HTTP Server", 15*time.Second, func(ctx context.Context) error {
		return pwaServer.Shutdown(ctx)
	})
	// Start the PWA server in a separate goroutine
	go func() {
		slog.Info("PWA Server starting on :8080")
		if err := pwaServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("PWA Server error", "error", err)
		}
	}()

	// Wait for shutdown signal
	err = gracedownManager.WaitForSignalAndShutdown()
	if err != nil {
		slog.Error("Error on graceful shutdown", "error", err)
		os.Exit(gracedown.ExitFailure)
	}

	slog.Info("Server stopped gracefully")

	os.Exit(gracedown.ExitSuccess)
}

// emergencyExit shuts down all registered components and exits with the given code.
func emergencyExit(manager *gracedown.Manager, exitCode int) {
	if shutdownErr := manager.EmergencyShutdown(); shutdownErr != nil {
		slog.Error("Emergency shutdown error", "error", shutdownErr)
	}
	os.Exit(exitCode)
}

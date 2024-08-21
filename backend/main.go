package main

import (
	"net/http"

	"auto/backend/handlers"
	"auto/config"
	"auto/dbmanager"
	"auto/flow"
	"auto/logger"
	"auto/model"
	"auto/websocket"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger := logger.NewLogger()
	defer logger.Sync()

	// Load configuration
	cfg, err := config.LoadConfig(".env")
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Initialize database manager
	dbManager := &dbmanager.DbManager{}
	if err := dbManager.Init(); err != nil {
		logger.Fatal("Failed to initialize database manager", zap.Error(err))
	}

	// Initialize instance manager
	instanceManager := model.NewInstanceManager(logger)

	// Initialize flow repository
	flowRepo := flow.NewFlowRepository(dbManager.Client, logger)

	// Initialize flow manager
	flowManager := flow.NewManager(dbManager.Client, flowRepo, logger, dbManager.Client)

	// Initialize handler
	handler := handlers.NewHandler(logger, dbManager, flowManager, instanceManager)

	// Set up Gin router
	r := gin.Default()

	// Register routes
	handlers.RegisterRoutes(r, handler)

	// WebSocket Route
	r.GET("/ws", func(c *gin.Context) {
		websocket.WebsocketHandler(c.Writer, c.Request)
	})

	// Start the server
	addr := ":" + cfg.ServerPort
	logger.Info("Starting server", zap.String("addr", addr))
	if err := http.ListenAndServe(addr, r); err != nil {
		logger.Fatal("Failed to start server", zap.Error(err))
	}
}

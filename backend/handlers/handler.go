package handlers

import (
	"net/http"
	"time"

	"auto/dbmanager"
	"auto/flow"
	"auto/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Handler struct {
	logger          *zap.Logger
	dbManager       *dbmanager.DbManager
	flowManager     *flow.Manager
	instanceManager *model.InstanceManager
}

func NewHandler(logger *zap.Logger, dbManager *dbmanager.DbManager, flowManager *flow.Manager, instanceManager *model.InstanceManager) *Handler {
	return &Handler{
		logger:          logger,
		dbManager:       dbManager,
		flowManager:     flowManager,
		instanceManager: instanceManager,
	}
}

// Flow Handlers
func (h *Handler) CreateFlowHandler(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newFlow := h.flowManager.CreateFlow(req.Name, "")
	if newFlow == nil {
		h.logger.Error("Failed to create flow")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create flow"})
		return
	}

	// Save flow to database
	dbFlow := dbmanager.DbFlow{
		ID:        dbmanager.NewNullString(newFlow.GetID()),
		Instances: dbmanager.NewNullString(newFlow.GetInstanceID()),
		Steps:     dbmanager.NewNullString(""), // Assuming steps are initially empty
		Status:    dbmanager.NewNullString("created"),
	}
	if err := h.dbManager.SaveFlow(dbFlow); err != nil {
		h.logger.Error("Failed to save flow to database", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save flow to database"})
		return
	}

	c.JSON(http.StatusOK, newFlow)
}

func (h *Handler) GetFlowsHandler(c *gin.Context) {
	flows := h.flowManager.GetFlows()
	c.JSON(http.StatusOK, flows)
}

func (h *Handler) DeleteFlowHandler(c *gin.Context) {
	id := c.Param("id")
	err := h.flowManager.DeleteFlow(id)
	if err != nil {
		h.logger.Error("Failed to delete flow", zap.String("flowID", id), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Delete flow from database
	if err := h.dbManager.DeleteFlow(id); err != nil {
		h.logger.Error("Failed to delete flow from database", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete flow from database"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *Handler) ExecuteFlowsHandler(c *gin.Context) {
	var req struct {
		FlowIDs []string `json:"flow_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	errors := h.flowManager.ExecuteFlowsConcurrently(req.FlowIDs, *h.instanceManager)
	if len(errors) > 0 {
		h.logger.Error("Failed to execute flows", zap.Errors("errors", errors))
		c.JSON(http.StatusInternalServerError, gin.H{"errors": errors})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "flows executed"})
}

// Instance Handlers
func (h *Handler) AddInstanceHandler(c *gin.Context) {
	var req struct {
		URL  string     `json:"url"`
		Auth model.Auth `json:"auth"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newInstance, err := h.instanceManager.CreateInstance(req.URL, req.Auth)
	if err != nil {
		h.logger.Error("Failed to create instance", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Save instance to database
	dbInstance := dbmanager.DbInstance{
		ID:       dbmanager.NewNullString(newInstance.ID),
		URL:      dbmanager.NewNullString(newInstance.URL),
		Auth:     dbmanager.NewNullString(""), // Assuming auth is stored as JSON string
		Status:   dbmanager.NewNullString(newInstance.Status),
		LastUsed: dbmanager.NewNullTime(time.Now()),
	}
	if err := h.dbManager.SaveInstance(dbInstance); err != nil {
		h.logger.Error("Failed to save instance to database", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save instance to database"})
		return
	}

	c.JSON(http.StatusOK, newInstance)
}

func (h *Handler) GetInstancesHandler(c *gin.Context) {
	instances := h.instanceManager.GetInstances()
	c.JSON(http.StatusOK, instances)
}

func (h *Handler) DeleteInstanceHandler(c *gin.Context) {
	id := c.Param("id")
	err := h.instanceManager.DeleteInstance(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Delete instance from database
	if err := h.dbManager.DeleteInstance(id); err != nil {
		h.logger.Error("Failed to delete instance from database", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete instance from database"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *Handler) StartInstancesHandler(c *gin.Context) {
	var req struct {
		InstanceIDs []string `json:"instance_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	errors := h.instanceManager.StartInstancesConcurrently(req.InstanceIDs)
	if len(errors) > 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"errors": errors})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "instances started"})
}

func (h *Handler) StopAllInstancesHandler(c *gin.Context) {
	errors := h.instanceManager.StopAllInstances()
	if len(errors) > 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"errors": errors})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "all instances stopped"})
}

func (h *Handler) StopInstanceHandler(c *gin.Context) {
	id := c.Param("id")
	err := h.instanceManager.StopInstance(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

func (h *Handler) UpdateInstanceStatusHandler(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.instanceManager.UpdateInstanceStatus(id, req.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *Handler) GetInstanceScreenshotHandler(c *gin.Context) {
	id := c.Param("id")
	screenshot, err := h.instanceManager.GetInstanceScreenshot(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "image/png", screenshot)
}

// RegisterRoutes registers all routes with the Gin engine
func RegisterRoutes(r *gin.Engine, handler *Handler) {
	// Middleware to inject logger into context
	r.Use(func(c *gin.Context) {
		c.Set("logger", handler.logger)
		c.Next()
	})

	// Instance routes
	r.POST("/api/v1/instances", handler.AddInstanceHandler)
	r.GET("/api/v1/instances", handler.GetInstancesHandler)
	r.DELETE("/api/v1/instances/:id", handler.DeleteInstanceHandler)
	r.POST("/api/v1/instances/start", handler.StartInstancesHandler)
	r.POST("/api/v1/instances/stop-all", handler.StopAllInstancesHandler)
	r.POST("/api/v1/instances/:id/stop", handler.StopInstanceHandler)
	r.PUT("/api/v1/instances/:id/status", handler.UpdateInstanceStatusHandler)
	r.GET("/api/v1/instances/:id/screenshot", handler.GetInstanceScreenshotHandler)

	// Flow routes
	r.POST("/api/v1/flows", handler.CreateFlowHandler)
	r.GET("/api/v1/flows", handler.GetFlowsHandler)
	r.DELETE("/api/v1/flows/:id", handler.DeleteFlowHandler)
	r.POST("/api/v1/flows/execute", handler.ExecuteFlowsHandler)
}

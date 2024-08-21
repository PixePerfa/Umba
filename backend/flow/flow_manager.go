package flow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sync"
	"text/template"
	"time"

	"auto/model"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type FlowRepository interface {
	CreateFlow(ctx context.Context, f Flow) error
	GetFlow(ctx context.Context, id string) (Flow, error)
	GetFlows(ctx context.Context) ([]Flow, error)
	UpdateFlow(ctx context.Context, f Flow) error
	DeleteFlow(ctx context.Context, id string) error
}

type Flow interface {
	GetID() string
	GetName() string
	GetInstanceID() string
	GetSteps() []Step
	SetSteps(steps []Step)
}

type Step struct {
	ID     string                 `json:"id"`
	Action string                 `json:"action"`
	Params map[string]interface{} `json:"params"`
}

type FlowImpl struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	InstanceID string `json:"instance_id"`
	Steps      []Step `json:"steps"`
}

func (f *FlowImpl) GetID() string {
	return f.ID
}

func (f *FlowImpl) GetName() string {
	return f.Name
}

func (f *FlowImpl) GetInstanceID() string {
	return f.InstanceID
}

func (f *FlowImpl) GetSteps() []Step {
	return f.Steps
}

func (f *FlowImpl) SetSteps(steps []Step) {
	f.Steps = steps
}

type Manager struct {
	flows  map[string]Flow
	mu     sync.RWMutex
	db     *redis.Client
	repo   FlowRepository
	logger *zap.Logger
	cache  *redis.Client
}

func NewManager(db *redis.Client, repo FlowRepository, logger *zap.Logger, cache *redis.Client) *Manager {
	m := &Manager{
		flows:  make(map[string]Flow),
		db:     db,
		repo:   repo,
		logger: logger,
		cache:  cache,
	}
	if err := m.loadFlowsFromDB(); err != nil {
		m.logger.Fatal("Failed to load flows from DB", zap.Error(err))
	}
	return m
}

func (m *Manager) loadFlowsFromDB() error {
	flows, err := m.repo.GetFlows(context.Background())
	if err != nil {
		m.logger.Error("Failed to load flows from DB", zap.Error(err))
		return err
	}
	for _, flow := range flows {
		m.flows[flow.GetID()] = flow
	}
	return nil
}

func (m *Manager) CreateFlow(name string, instanceID string) Flow {
	flow := &FlowImpl{
		ID:         uuid.New().String(),
		Name:       name,
		InstanceID: instanceID,
		Steps:      []Step{},
	}

	m.mu.Lock()
	m.flows[flow.ID] = flow
	m.mu.Unlock()

	// Store flow details in Redis
	flowJSON, _ := json.Marshal(flow)
	m.cache.HSet(context.Background(), "flows", flow.ID, flowJSON)

	err := m.repo.CreateFlow(context.Background(), flow)
	if err != nil {
		m.logger.Error("Failed to create flow in DB", zap.Error(err))
		return nil
	}

	return flow
}

func (m *Manager) UpdateFlow(flow Flow) error {
	m.mu.Lock()
	m.flows[flow.GetID()] = flow
	m.mu.Unlock()

	// Update flow details in Redis
	flowJSON, _ := json.Marshal(flow)
	m.cache.HSet(context.Background(), "flows", flow.GetID(), flowJSON)

	return m.repo.UpdateFlow(context.Background(), flow)
}

func (m *Manager) DeleteFlow(id string) error {
	m.mu.Lock()
	delete(m.flows, id)
	m.mu.Unlock()

	// Remove flow from Redis
	m.cache.HDel(context.Background(), "flows", id)

	return m.repo.DeleteFlow(context.Background(), id)
}

func (m *Manager) GetFlows() []Flow {
	m.mu.RLock()
	defer m.mu.RUnlock()

	flows := make([]Flow, 0, len(m.flows))
	for _, flow := range m.flows {
		flows = append(flows, flow)
	}
	return flows
}

func (m *Manager) AddStep(flowID string, action string, params map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	flow, exists := m.flows[flowID]
	if !exists {
		return fmt.Errorf("flow not found: %s", flowID)
	}

	step := Step{
		ID:     uuid.New().String(),
		Action: action,
		Params: params,
	}

	steps := flow.GetSteps()
	steps = append(steps, step)
	flow.SetSteps(steps)

	return m.repo.UpdateFlow(context.Background(), flow)
}

func (m *Manager) SaveToFile(filename string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m.flows, "", "  ")
	if err != nil {
		m.logger.Error("Failed to marshal flows", zap.Error(err))
		return err
	}

	return ioutil.WriteFile(filename, data, 0644)
}

func (m *Manager) LoadFromFile(filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		m.logger.Error("Failed to read flows file", zap.Error(err))
		return err
	}

	var flows map[string]Flow
	if err := json.Unmarshal(data, &flows); err != nil {
		m.logger.Error("Failed to unmarshal flows", zap.Error(err))
		return err
	}

	m.mu.Lock()
	m.flows = flows
	m.mu.Unlock()

	return nil
}

func (m *Manager) ExecuteFlow(flowID string, instanceManager model.InstanceManager) error {
	m.mu.RLock()
	flow, exists := m.flows[flowID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("flow not found: %s", flowID)
	}

	instance, err := instanceManager.GetInstance(flow.GetInstanceID())
	if err != nil {
		return fmt.Errorf("failed to get instance: %w", err)
	}

	instanceResponses := make(map[string]string)

	for _, step := range flow.GetSteps() {
		switch step.Action {
		case "template":
			tmpl, err := template.New("response").Parse(step.Params["template"].(string))
			if err != nil {
				return err
			}
			var result bytes.Buffer
			err = tmpl.Execute(&result, instanceResponses)
			if err != nil {
				return err
			}
			instanceResponses["templateResult"] = result.String()
		default:
			result, err := instance.Execute(step.Action, step.Params)
			if err != nil {
				m.logger.Error("Step execution failed", zap.String("flowID", flowID), zap.String("stepID", step.ID), zap.Error(err))
				return fmt.Errorf("failed to execute step %s: %w", step.ID, err)
			}
			instanceResponses[step.ID] = result
		}
	}

	m.logger.Info("Flow executed successfully", zap.String("flowID", flowID))
	return nil
}

func (m *Manager) ExecuteFlowsConcurrently(flowIDs []string, instanceManager model.InstanceManager) []error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(flowIDs))

	for _, id := range flowIDs {
		wg.Add(1)
		go func(flowID string) {
			defer wg.Done()
			if err := m.ExecuteFlow(flowID, instanceManager); err != nil {
				errChan <- fmt.Errorf("failed to execute flow %s: %w", flowID, err)
			}
		}(id)
	}

	wg.Wait()
	close(errChan)

	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	return errors
}

func (m *Manager) GetFlowFromCache(flowID string) (Flow, error) {
	cachedFlow, err := m.cache.Get(context.Background(), flowID).Bytes()
	if err != nil {
		return nil, err
	}

	var flow FlowImpl
	err = json.Unmarshal(cachedFlow, &flow)
	if err != nil {
		return nil, err
	}

	return &flow, nil
}

func (m *Manager) CacheFlow(flow Flow) error {
	flowData, err := json.Marshal(flow)
	if err != nil {
		return err
	}

	return m.cache.Set(context.Background(), flow.GetID(), flowData, 5*time.Minute).Err()
}

// FlowRepositoryImpl implements the FlowRepository interface
type FlowRepositoryImpl struct {
	db     *redis.Client
	logger *zap.Logger
}

func NewFlowRepository(db *redis.Client, logger *zap.Logger) *FlowRepositoryImpl {
	return &FlowRepositoryImpl{db: db, logger: logger}
}

func (r *FlowRepositoryImpl) CreateFlow(ctx context.Context, f Flow) error {
	steps, err := json.Marshal(f.GetSteps())
	if err != nil {
		return err
	}
	flow := FlowImpl{
		ID:         f.GetID(),
		Name:       f.GetName(),
		InstanceID: f.GetInstanceID(),
		Steps:      []Step{},
	}
	err = json.Unmarshal(steps, &flow.Steps)
	if err != nil {
		return err
	}
	data, err := json.Marshal(flow)
	if err != nil {
		return err
	}
	return r.db.Set(ctx, fmt.Sprintf("flow:%s", flow.ID), data, 0).Err()
}

func (r *FlowRepositoryImpl) GetFlow(ctx context.Context, id string) (Flow, error) {
	result, err := r.db.Get(ctx, fmt.Sprintf("flow:%s", id)).Result()
	if err != nil {
		return nil, err
	}
	var flow FlowImpl
	err = json.Unmarshal([]byte(result), &flow)
	if err != nil {
		return nil, err
	}
	return &flow, nil
}

func (r *FlowRepositoryImpl) GetFlows(ctx context.Context) ([]Flow, error) {
	keys, err := r.db.Keys(ctx, "flow:*").Result()
	if err != nil {
		return nil, err
	}
	var flows []Flow
	for _, key := range keys {
		result, err := r.db.Get(ctx, key).Result()
		if err != nil {
			return nil, err
		}
		var flow FlowImpl
		err = json.Unmarshal([]byte(result), &flow)
		if err != nil {
			return nil, err
		}
		flows = append(flows, &flow)
	}
	return flows, nil
}

func (r *FlowRepositoryImpl) UpdateFlow(ctx context.Context, f Flow) error {
	steps, err := json.Marshal(f.GetSteps())
	if err != nil {
		return err
	}
	flow := FlowImpl{
		ID:         f.GetID(),
		Name:       f.GetName(),
		InstanceID: f.GetInstanceID(),
		Steps:      []Step{},
	}
	err = json.Unmarshal(steps, &flow.Steps)
	if err != nil {
		return err
	}
	data, err := json.Marshal(flow)
	if err != nil {
		return err
	}
	return r.db.Set(ctx, fmt.Sprintf("flow:%s", flow.ID), data, 0).Err()
}

func (r *FlowRepositoryImpl) DeleteFlow(ctx context.Context, id string) error {
	return r.db.Del(ctx, fmt.Sprintf("flow:%s", id)).Err()
}

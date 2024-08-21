package dbmanager

import (
	"auto/config"
	"auto/logger"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type DbManager struct {
	Client *redis.Client
}

// NewNullString creates a new sql.NullString.
func NewNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// NewNullTime creates a new sql.NullTime.
func NewNullTime(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: !t.IsZero()}
}

type DbInstance struct {
	ID       sql.NullString `json:"id"`
	URL      sql.NullString `json:"url"`
	Auth     sql.NullString `json:"auth"`
	Status   sql.NullString `json:"status"`
	LastUsed sql.NullTime   `json:"last_used"`
}

type DbFlow struct {
	ID        sql.NullString `json:"id"`
	Instances sql.NullString `json:"instances"`
	Steps     sql.NullString `json:"steps"`
	Status    sql.NullString `json:"status"`
}

type DbAction struct {
	ID        string    `json:"id"`
	Instance  string    `json:"instance"`
	Action    string    `json:"action"`
	Timestamp time.Time `json:"timestamp"`
}

type DbMessage struct {
	ID        string    `json:"id"`
	Instance  string    `json:"instance"`
	Flow      string    `json:"flow"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Init initializes the database connection
func (Dm *DbManager) Init() error {
	cfg, err := config.LoadConfig(".env")
	if err != nil {
		return fmt.Errorf("failed to load configuration: %v", err)
	}

	Dm.Client = redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
		DB:   cfg.RedisDB,
	})

	_, err = Dm.Client.Ping(context.Background()).Result()
	if err != nil {
		return err
	}

	logger.Info("[DB] connect success")
	return nil
}

// GetInstance retrieves an instance by ID
func (Dm *DbManager) GetInstance(id string) (DbInstance, error) {
	result, err := Dm.Client.Get(context.Background(), fmt.Sprintf("instance:%s", id)).Result()
	if err != nil {
		logger.Error("get instance error", zap.Error(err))
		return DbInstance{}, err
	}

	var instance DbInstance
	err = json.Unmarshal([]byte(result), &instance)
	if err != nil {
		logger.Error("unmarshal instance error", zap.Error(err))
		return DbInstance{}, err
	}

	return instance, nil
}

// SaveInstance saves an instance to the database
func (Dm *DbManager) SaveInstance(instance DbInstance) error {
	data, err := json.Marshal(instance)
	if err != nil {
		logger.Error("marshal instance error", zap.Error(err))
		return err
	}

	err = Dm.Client.Set(context.Background(), fmt.Sprintf("instance:%s", instance.ID.String), data, 0).Err()
	if err != nil {
		logger.Error("save instance error", zap.Error(err))
		return err
	}

	return nil
}

// UpdateInstance updates an instance in the database
func (Dm *DbManager) UpdateInstance(instance DbInstance) error {
	data, err := json.Marshal(instance)
	if err != nil {
		logger.Error("marshal instance error", zap.Error(err))
		return err
	}

	err = Dm.Client.Set(context.Background(), fmt.Sprintf("instance:%s", instance.ID.String), data, 0).Err()
	if err != nil {
		logger.Error("update instance error", zap.Error(err))
		return err
	}

	return nil
}

// DeleteInstance deletes an instance by ID
func (Dm *DbManager) DeleteInstance(id string) error {
	err := Dm.Client.Del(context.Background(), fmt.Sprintf("instance:%s", id)).Err()
	if err != nil {
		logger.Error("delete instance error", zap.Error(err))
		return err
	}

	return nil
}

// GetFlow retrieves a flow by ID
func (Dm *DbManager) GetFlow(id string) (DbFlow, error) {
	result, err := Dm.Client.Get(context.Background(), fmt.Sprintf("flow:%s", id)).Result()
	if err != nil {
		logger.Error("get flow error", zap.Error(err))
		return DbFlow{}, err
	}

	var flow DbFlow
	err = json.Unmarshal([]byte(result), &flow)
	if err != nil {
		logger.Error("unmarshal flow error", zap.Error(err))
		return DbFlow{}, err
	}

	return flow, nil
}

// SaveFlow saves a flow to the database
func (Dm *DbManager) SaveFlow(flow DbFlow) error {
	data, err := json.Marshal(flow)
	if err != nil {
		logger.Error("marshal flow error", zap.Error(err))
		return err
	}

	err = Dm.Client.Set(context.Background(), fmt.Sprintf("flow:%s", flow.ID.String), data, 0).Err()
	if err != nil {
		logger.Error("save flow error", zap.Error(err))
		return err
	}

	return nil
}

// UpdateFlow updates a flow in the database
func (Dm *DbManager) UpdateFlow(flow DbFlow) error {
	data, err := json.Marshal(flow)
	if err != nil {
		logger.Error("marshal flow error", zap.Error(err))
		return err
	}

	err = Dm.Client.Set(context.Background(), fmt.Sprintf("flow:%s", flow.ID.String), data, 0).Err()
	if err != nil {
		logger.Error("update flow error", zap.Error(err))
		return err
	}

	return nil
}

// DeleteFlow deletes a flow by ID
func (Dm *DbManager) DeleteFlow(id string) error {
	err := Dm.Client.Del(context.Background(), fmt.Sprintf("flow:%s", id)).Err()
	if err != nil {
		logger.Error("delete flow error", zap.Error(err))
		return err
	}

	return nil
}

// SaveAction saves an action to the database
func (Dm *DbManager) SaveAction(action DbAction) error {
	data, err := json.Marshal(action)
	if err != nil {
		logger.Error("marshal action error", zap.Error(err))
		return err
	}

	err = Dm.Client.Set(context.Background(), fmt.Sprintf("action:%s", action.ID), data, 0).Err()
	if err != nil {
		logger.Error("save action error", zap.Error(err))
		return err
	}

	return nil
}

// GetActions retrieves actions by instance ID
func (Dm *DbManager) GetActions(instanceID string) ([]DbAction, error) {
	keys, err := Dm.Client.Keys(context.Background(), fmt.Sprintf("action:%s:*", instanceID)).Result()
	if err != nil {
		logger.Error("get actions keys error", zap.Error(err))
		return nil, err
	}

	var actions []DbAction
	for _, key := range keys {
		result, err := Dm.Client.Get(context.Background(), key).Result()
		if err != nil {
			logger.Error("get action error", zap.Error(err))
			continue
		}

		var action DbAction
		err = json.Unmarshal([]byte(result), &action)
		if err != nil {
			logger.Error("unmarshal action error", zap.Error(err))
			continue
		}

		actions = append(actions, action)
	}

	return actions, nil
}

// SaveMessage saves a message to the database
func (Dm *DbManager) SaveMessage(message DbMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		logger.Error("marshal message error", zap.Error(err))
		return err
	}

	err = Dm.Client.Set(context.Background(), fmt.Sprintf("message:%s", message.ID), data, 0).Err()
	if err != nil {
		logger.Error("save message error", zap.Error(err))
		return err
	}

	return nil
}

// GetMessagesByInstance retrieves messages by instance ID
func (Dm *DbManager) GetMessagesByInstance(instanceID string) ([]DbMessage, error) {
	keys, err := Dm.Client.Keys(context.Background(), fmt.Sprintf("message:%s:*", instanceID)).Result()
	if err != nil {
		logger.Error("get messages keys error", zap.Error(err))
		return nil, err
	}

	var messages []DbMessage
	for _, key := range keys {
		result, err := Dm.Client.Get(context.Background(), key).Result()
		if err != nil {
			logger.Error("get message error", zap.Error(err))
			continue
		}

		var message DbMessage
		err = json.Unmarshal([]byte(result), &message)
		if err != nil {
			logger.Error("unmarshal message error", zap.Error(err))
			continue
		}

		messages = append(messages, message)
	}

	return messages, nil
}

// GetMessagesByFlow retrieves messages by flow ID
func (Dm *DbManager) GetMessagesByFlow(flowID string) ([]DbMessage, error) {
	keys, err := Dm.Client.Keys(context.Background(), fmt.Sprintf("message:%s:*", flowID)).Result()
	if err != nil {
		logger.Error("get messages keys error", zap.Error(err))
		return nil, err
	}

	var messages []DbMessage
	for _, key := range keys {
		result, err := Dm.Client.Get(context.Background(), key).Result()
		if err != nil {
			logger.Error("get message error", zap.Error(err))
			continue
		}

		var message DbMessage
		err = json.Unmarshal([]byte(result), &message)
		if err != nil {
			logger.Error("unmarshal message error", zap.Error(err))
			continue
		}

		messages = append(messages, message)
	}

	return messages, nil
}

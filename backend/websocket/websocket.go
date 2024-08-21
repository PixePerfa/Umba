package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Instance struct {
	ID           string
	URL          string
	Auth         *Auth
	Status       string
	Context      context.Context
	Cancel       context.CancelFunc
	ChromeCtx    context.Context
	ChromeCancel context.CancelFunc
}

type Auth struct {
	Email    string
	Password string
}

var instances = make(map[string]*Instance)
var instancesLock sync.Mutex
var logger *zap.Logger
var rdb *redis.Client // Redis client instance

func init() {
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
	// Initialize Redis client
	rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Update with your Redis server address
		DB:   0,                // Update with your Redis database number
	})
}

func WebsocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("Failed to upgrade to websocket", zap.Error(err))
		return
	}
	defer conn.Close()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			logger.Error("Failed to read message", zap.Error(err))
			break
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			logger.Error("Failed to unmarshal message", zap.Error(err))
			continue
		}

		handleMessage(conn, msg)
	}
}

func handleMessage(conn *websocket.Conn, msg map[string]interface{}) {
	action, ok := msg["action"].(string)
	if !ok {
		logger.Error("Invalid action")
		return
	}

	switch action {
	case "createInstance":
		createInstance(conn, msg)
	case "startInstance":
		startInstance(conn, msg)
	case "stopInstance":
		stopInstance(conn, msg)
	case "deleteInstance":
		deleteInstance(conn, msg)
	case "debugInstance":
		debugInstance(conn, msg)
	default:
		logger.Error("Unknown action", zap.String("action", action))
	}
}

func createInstance(conn *websocket.Conn, msg map[string]interface{}) {
	url, ok := msg["url"].(string)
	if !ok {
		sendError(conn, "URL is required")
		return
	}

	auth := &Auth{}
	if requiresAuth, ok := msg["requiresAuth"].(bool); ok && requiresAuth {
		email, ok := msg["email"].(string)
		if !ok {
			sendError(conn, "Email is required")
			return
		}
		password, ok := msg["password"].(string)
		if !ok {
			sendError(conn, "Password is required")
			return
		}
		auth = &Auth{Email: email, Password: password}
	}

	instance := &Instance{
		ID:     generateID(),
		URL:    url,
		Auth:   auth,
		Status: "Off",
	}

	instancesLock.Lock()
	instances[instance.ID] = instance
	instancesLock.Unlock()

	// Store instance details in Redis
	instanceJSON, _ := json.Marshal(instance)
	rdb.HSet(context.Background(), "instances", instance.ID, instanceJSON)

	sendSuccess(conn, map[string]interface{}{
		"message": "Instance created",
		"instance": map[string]interface{}{
			"id":     instance.ID,
			"url":    instance.URL,
			"status": instance.Status,
		},
	})
}

func startInstance(conn *websocket.Conn, msg map[string]interface{}) {
	id, ok := msg["id"].(string)
	if !ok {
		sendError(conn, "Instance ID is required")
		return
	}

	instancesLock.Lock()
	instance, ok := instances[id]
	instancesLock.Unlock()

	if !ok {
		sendError(conn, "Instance not found")
		return
	}

	if instance.Status == "On" {
		sendError(conn, "Instance is already running")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	chromeCtx, chromeCancel := chromedp.NewContext(ctx)

	instance.Context = ctx
	instance.Cancel = cancel
	instance.ChromeCtx = chromeCtx
	instance.ChromeCancel = chromeCancel
	instance.Status = "On"

	go func() {
		if err := chromedp.Run(chromeCtx, navigateAndAuthenticate(instance)); err != nil {
			logger.Error("Failed to start instance", zap.Error(err))
			instance.Status = "Off"
			return
		}
		logger.Info("Instance started", zap.String("id", instance.ID))
	}()

	// Update instance status in Redis
	instanceJSON, _ := json.Marshal(instance)
	rdb.HSet(context.Background(), "instances", id, instanceJSON)

	sendSuccess(conn, map[string]interface{}{
		"message": "Instance started",
		"instance": map[string]interface{}{
			"id":     instance.ID,
			"url":    instance.URL,
			"status": instance.Status,
		},
	})
}

func stopInstance(conn *websocket.Conn, msg map[string]interface{}) {
	id, ok := msg["id"].(string)
	if !ok {
		sendError(conn, "Instance ID is required")
		return
	}

	instancesLock.Lock()
	instance, ok := instances[id]
	instancesLock.Unlock()

	if !ok {
		sendError(conn, "Instance not found")
		return
	}

	if instance.Status == "Off" {
		sendError(conn, "Instance is already stopped")
		return
	}

	instance.Cancel()
	instance.ChromeCancel()
	instance.Status = "Off"

	// Update instance status in Redis
	instanceJSON, _ := json.Marshal(instance)
	rdb.HSet(context.Background(), "instances", id, instanceJSON)

	sendSuccess(conn, map[string]interface{}{
		"message": "Instance stopped",
		"instance": map[string]interface{}{
			"id":     instance.ID,
			"url":    instance.URL,
			"status": instance.Status,
		},
	})
}

func deleteInstance(conn *websocket.Conn, msg map[string]interface{}) {
	id, ok := msg["id"].(string)
	if !ok {
		sendError(conn, "Instance ID is required")
		return
	}

	instancesLock.Lock()
	_, ok = instances[id]
	if !ok {
		instancesLock.Unlock()
		sendError(conn, "Instance not found")
		return
	}
	delete(instances, id)
	instancesLock.Unlock()

	// Remove instance from Redis
	rdb.HDel(context.Background(), "instances", id)

	sendSuccess(conn, map[string]interface{}{
		"message": "Instance deleted",
		"id":      id,
	})
}

func debugInstance(conn *websocket.Conn, msg map[string]interface{}) {
	id, ok := msg["id"].(string)
	if !ok {
		sendError(conn, "Instance ID is required")
		return
	}

	instancesLock.Lock()
	instance, ok := instances[id]
	instancesLock.Unlock()

	if !ok {
		sendError(conn, "Instance not found")
		return
	}

	var buf []byte
	if err := chromedp.Run(instance.ChromeCtx, chromedp.CaptureScreenshot(&buf)); err != nil {
		sendError(conn, "Failed to capture screenshot")
		return
	}

	sendSuccess(conn, map[string]interface{}{
		"message":    "Instance debug screenshot",
		"screenshot": buf,
	})
}

func sendError(conn *websocket.Conn, message string) {
	conn.WriteJSON(map[string]interface{}{
		"status":  "error",
		"message": message,
	})
}

func sendSuccess(conn *websocket.Conn, data map[string]interface{}) {
	conn.WriteJSON(map[string]interface{}{
		"status": "success",
		"data":   data,
	})
}

func generateID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

func navigateAndAuthenticate(instance *Instance) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.Navigate(instance.URL),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if instance.Auth != nil {
				if err := chromedp.SendKeys(`input[name="email"]`, instance.Auth.Email).Do(ctx); err != nil {
					return err
				}
				if err := chromedp.SendKeys(`input[name="password"]`, instance.Auth.Password).Do(ctx); err != nil {
					return err
				}
				if err := chromedp.Click(`button[type="submit"]`).Do(ctx); err != nil {
					return err
				}
			}
			return nil
		}),
	}
}

// NetworkIdleListener listens for network idle events
func NetworkIdleListener(ctx context.Context, networkIdleTimeout, totalTimeout time.Duration) chan IdleEvent {
	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan IdleEvent, 1) // buffer to prevent blocking
	var idleTimer *time.Timer
	go func() {
		<-time.After(totalTimeout)
		ch <- IdleEvent{IsIdle: false}
		cancel()
		close(ch)
	}()
	listener := newNetworkIdleListener(ch, networkIdleTimeout, idleTimer)
	chromedp.ListenTarget(ctx, listener)
	return ch
}

// NetworkIdlePermanentListener listens for network idle events permanently
func NetworkIdlePermanentListener(ctx context.Context, networkIdleTimeout time.Duration) (chan IdleEvent, func()) {
	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan IdleEvent, 1) // buffer to prevent blocking
	var idleTimer *time.Timer
	listener := newNetworkIdleListener(ch, networkIdleTimeout, idleTimer)
	chromedp.ListenTarget(ctx, listener)
	cancelFunc := func() {
		cancel()
		close(ch)
	}

	return ch, cancelFunc
}

// newNetworkIdleListener creates a new network idle listener
func newNetworkIdleListener(ch chan IdleEvent, networkIdleTimeout time.Duration, idleTimer *time.Timer) func(interface{}) {
	return func(ev interface{}) {
		if _, ok := ev.(*cdproto.Message); ok {
			return
		}

		if _, ok := ev.(*network.EventRequestWillBeSent); ok {
			if idleTimer != nil {
				idleTimer.Stop()
				idleTimer = nil
			}
		}

		if ev, ok := ev.(*page.EventLifecycleEvent); ok && ev.Name == "networkIdle" {
			if idleTimer == nil {
				idleTimer = time.AfterFunc(networkIdleTimeout, func() {
					ch <- IdleEvent{IsIdle: true}
				})
			} else {
				idleTimer.Reset(networkIdleTimeout)
			}
		}
	}
}

// IdleEvent represents an idle event
type IdleEvent struct {
	IsIdle bool
}

package model

import (
	"auto/websocket"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

var logger *zap.Logger
var instances = make(map[string]*Instance)
var instancesLock sync.Mutex
var rdb *redis.Client

type ChromeDPContext interface {
	Run(context.Context, ...chromedp.Action) error
	NewContext(context.Context) (context.Context, context.CancelFunc)
}

type DefaultChromeDPContext struct{}

func (d *DefaultChromeDPContext) Run(ctx context.Context, actions ...chromedp.Action) error {
	return chromedp.Run(ctx, actions...)
}

func (d *DefaultChromeDPContext) NewContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return chromedp.NewContext(ctx)
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
	Elements     *Elements
	chrome       ChromeDPContext
}

type Auth struct {
	Email    string
	Password string
}

type Elements struct {
	UsernameSel string
	PasswordSel string
	SubmitSel   string
}

func init() {
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
	rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})
}

func GenerateID() string {
	return fmt.Sprintf("%x", md5.Sum([]byte(time.Now().String())))
}

func CreateInstance(url string, auth *Auth, elements *Elements, chrome ChromeDPContext) *Instance {
	id := GenerateID()
	instance := &Instance{
		ID:       id,
		URL:      url,
		Auth:     auth,
		Status:   "Off",
		Elements: elements,
		chrome:   chrome,
	}
	instancesLock.Lock()
	instances[id] = instance
	instancesLock.Unlock()

	// Store instance details in Redis
	instanceJSON, _ := json.Marshal(instance)
	rdb.HSet(context.Background(), "instances", id, instanceJSON)

	return instance
}

func StartInstance(id string) error {
	instancesLock.Lock()
	instance, ok := instances[id]
	instancesLock.Unlock()
	if !ok {
		return errors.New("instance not found")
	}
	if instance.Status == "On" {
		return errors.New("instance is already running")
	}
	ctx, cancel := instance.chrome.NewContext(context.Background())
	instance.Context = ctx
	instance.Cancel = cancel
	instance.ChromeCtx, instance.ChromeCancel = ctx, cancel
	instance.Status = "On"
	go func() {
		if err := instance.chrome.Run(ctx, navigateAndAuthenticate(instance)); err != nil {
			logger.Error("Failed to start instance", zap.Error(err))
			instance.Status = "Off"
			return
		}
		logger.Info("Instance started", zap.String("id", instance.ID))
	}()

	// Update instance status in Redis
	instanceJSON, _ := json.Marshal(instance)
	rdb.HSet(context.Background(), "instances", id, instanceJSON)

	return nil
}

func StopInstance(id string) error {
	instancesLock.Lock()
	instance, ok := instances[id]
	instancesLock.Unlock()
	if !ok {
		return errors.New("instance not found")
	}
	if instance.Status == "Off" {
		return errors.New("instance is already stopped")
	}
	instance.Cancel()
	instance.ChromeCancel()
	instance.Status = "Off"

	// Update instance status in Redis
	instanceJSON, _ := json.Marshal(instance)
	rdb.HSet(context.Background(), "instances", id, instanceJSON)

	return nil
}

func DeleteInstance(id string) error {
	instancesLock.Lock()
	defer instancesLock.Unlock()
	if _, ok := instances[id]; !ok {
		return errors.New("instance not found")
	}
	delete(instances, id)

	// Remove instance from Redis
	rdb.HDel(context.Background(), "instances", id)

	return nil
}

func DebugInstance(id string) ([]byte, error) {
	instancesLock.Lock()
	instance, ok := instances[id]
	instancesLock.Unlock()
	if !ok {
		return nil, errors.New("instance not found")
	}
	var buf []byte
	if err := instance.chrome.Run(instance.ChromeCtx, chromedp.CaptureScreenshot(&buf)); err != nil {
		return nil, err
	}
	return buf, nil
}

func navigateAndAuthenticate(instance *Instance) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.Navigate(instance.URL),
		chromedp.WaitVisible(instance.Elements.UsernameSel),
		chromedp.SendKeys(instance.Elements.UsernameSel, instance.Auth.Email),
		chromedp.Click(instance.Elements.PasswordSel),
		chromedp.WaitVisible(instance.Elements.PasswordSel),
		chromedp.SendKeys(instance.Elements.PasswordSel, instance.Auth.Password),
		chromedp.Click(instance.Elements.SubmitSel),
	}
}

func SendMessage(conn *websocket.Conn, status int, message interface{}, instanceID string) error {
	return conn.WriteJSON(map[string]interface{}{
		"status":   status,
		"message":  message,
		"instance": instanceID,
	})
}

func SaveCrawOutput(resultList map[string][]interface{}, filePath string) error {
	data, err := json.Marshal(resultList)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filePath, data, 0644)
}

func ParseURL(sourceURL string) (*url.URL, error) {
	u, err := url.Parse(sourceURL)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func EscapePercentSign(raw string) string {
	return strings.ReplaceAll(raw, "%", "%25")
}

func DealMultipart(contentType, ruleBody string) (string, error) {
	re := regexp.MustCompile(`(?m)multipart\/form-Data; boundary=(.*)`)
	match := re.FindStringSubmatch(contentType)
	if len(match) != 2 {
		return "", errors.New("no boundary in content-type")
	}
	boundary := "--" + match[1]
	multiPartContent := ""
	multiFile := strings.Split(ruleBody, boundary)
	if len(multiFile) == 0 {
		return "", errors.New("ruleBody.Body multi content format err")
	}
	for _, singleFile := range multiFile {
		spliteTmp := strings.Split(singleFile, "\n\n")
		if len(spliteTmp) == 2 {
			fileHeader := spliteTmp[0]
			fileBody := spliteTmp[1]
			fileHeader = strings.Replace(fileHeader, "\n", "\r\n", -1)
			multiPartContent += boundary + fileHeader + "\r\n\r\n" + strings.TrimRight(fileBody, "\n") + "\r\n"
		}
	}
	multiPartContent += boundary + "--" + "\r\n"
	return multiPartContent, nil
}

// Define the missing types and variables

type Options struct {
	Headers  map[string]interface{}
	PostData string
}

type URL struct {
	url.URL
}

type Request struct {
	URL             *URL
	Method          string
	Headers         map[string]interface{}
	PostData        string
	RedirectionFlag bool
}

var supportContentType = []string{
	"application/json",
	"application/x-www-form-urlencoded",
	"multipart/form-data",
}

func GetRequest(method string, URL *URL, options ...Options) Request {
	var req Request
	req.URL = URL
	req.Method = strings.ToUpper(method)
	if len(options) != 0 {
		option := options[0]
		if option.Headers != nil {
			req.Headers = option.Headers
		}
		if option.PostData != "" {
			req.PostData = option.PostData
		}
	} else {
		req.Headers = map[string]interface{}{}
	}
	return req
}

func (req *Request) FormatPrint() {
	var tempStr = req.Method
	tempStr += " " + req.URL.String() + " HTTP/1.1\r\n"
	for k, v := range req.Headers {
		tempStr += k + ": " + v.(string) + "\r\n"
	}
	tempStr += "\r\n"
	if req.Method == "POST" {
		tempStr += req.PostData
	}
	fmt.Println(tempStr)
}

func (req *Request) SimplePrint() {
	var tempStr = req.Method
	tempStr += " " + req.URL.String() + " "
	if req.Method == "POST" {
		tempStr += req.PostData
	}
	fmt.Println(tempStr)
}

func (req *Request) SimpleFormat() string {
	var tempStr = req.Method
	tempStr += " " + req.URL.String() + " "
	if req.Method == "POST" {
		tempStr += req.PostData
	}
	return tempStr
}

func (req *Request) NoHeaderId() string {
	h := md5.New()
	h.Write([]byte(req.Method + req.URL.String() + req.PostData))
	return hex.EncodeToString(h.Sum(nil))
}

func (req *Request) UniqueId() string {
	if req.RedirectionFlag {
		h := md5.New()
		h.Write([]byte(req.NoHeaderId() + "Redirection"))
		return hex.EncodeToString(h.Sum(nil))
	} else {
		return req.NoHeaderId()
	}
}

func (req *Request) PostDataMap() map[string]interface{} {
	contentType, err := req.getContentType()
	if err != nil {
		return map[string]interface{}{
			"key": req.PostData,
		}
	}
	if strings.HasPrefix(contentType, "application/json") {
		var result map[string]interface{}
		err = json.Unmarshal([]byte(req.PostData), &result)
		if err != nil {
			return map[string]interface{}{
				"key": req.PostData,
			}
		} else {
			return result
		}
	} else if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		var result = map[string]interface{}{}
		r, err := url.ParseQuery(req.PostData)
		if err != nil {
			return map[string]interface{}{
				"key": req.PostData,
			}
		} else {
			for key, value := range r {
				if len(value) == 1 {
					result[key] = value[0]
				} else {
					result[key] = value
				}
			}
			return result
		}
	} else {
		return map[string]interface{}{
			"key": req.PostData,
		}
	}
}

func (req *Request) QueryMap() map[string][]string {
	return req.URL.Query()
}

func (req *Request) getContentType() (string, error) {
	headers := req.Headers
	var contentType string
	if ct, ok := headers["Content-Type"]; ok {
		contentType = ct.(string)
	} else if ct, ok := headers["Content-type"]; ok {
		contentType = ct.(string)
	} else if ct, ok := headers["content-type"]; ok {
		contentType = ct.(string)
	} else {
		return "", errors.New("no content-type")
	}
	for _, ct := range supportContentType {
		if strings.HasPrefix(contentType, ct) {
			return contentType, nil
		}
	}
	return "", errors.New("dont support such content-type:" + contentType)
}

func UrlParse(sourceUrl string) (*url.URL, error) {
	u, err := url.Parse(sourceUrl)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func GetUrl(_url string, parentUrls ...URL) (*URL, error) {
	var u URL
	_url, err := u.parse(_url, parentUrls...)
	if err != nil {
		return nil, err
	}
	if len(parentUrls) == 0 {
		_u, err := UrlParse(_url)
		if err != nil {
			return nil, err
		}
		u = URL{*_u}
		if u.Path == "" {
			u.Path = "/"
		}
	} else {
		pUrl := parentUrls[0]
		_u, err := pUrl.Parse(_url)
		if err != nil {
			return nil, err
		}
		u = URL{*_u}
		if u.Path == "" {
			u.Path = "/"
		}
	}
	fixPath := regexp.MustCompile("^/{2,}")
	if fixPath.MatchString(u.Path) {
		u.Path = fixPath.ReplaceAllString(u.Path, "/")
	}
	return &u, nil
}

func (u *URL) parse(_url string, parentUrls ...URL) (string, error) {
	_url = strings.Trim(_url, " ")
	if len(_url) == 0 {
		return "", errors.New("invalid url, length 0")
	}
	if strings.Count(_url, "#") > 1 {
		_url = regexp.MustCompile(`#+`).ReplaceAllString(_url, "#")
	}
	if len(parentUrls) == 0 {
		return _url, nil
	}
	if strings.HasPrefix(_url, "http://") || strings.HasPrefix(_url, "https://") {
		return _url, nil
	} else if strings.HasPrefix(_url, "javascript:") {
		return "", errors.New("invalid url, javascript protocol")
	} else if strings.HasPrefix(_url, "mailto:") {
		return "", errors.New("invalid url, mailto protocol")
	}
	return _url, nil
}

func (u *URL) QueryMap() map[string]interface{} {
	queryMap := map[string]interface{}{}
	for key, value := range u.Query() {
		if len(value) == 1 {
			queryMap[key] = value[0]
		} else {
			queryMap[key] = value
		}
	}
	return queryMap
}

func (u *URL) NoQueryUrl() string {
	return fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Path)
}

func (u *URL) NoFragmentUrl() string {
	return strings.Replace(u.String(), u.Fragment, "", -1)
}

func (u *URL) NoSchemeFragmentUrl() string {
	return fmt.Sprintf("://%s%s", u.Host, u.Path)
}

func (u *URL) NavigationUrl() string {
	return u.NoSchemeFragmentUrl()
}

func (u *URL) RootDomain() string {
	domain := u.Hostname()
	if strings.Count(domain, ".") == 1 {
		return domain
	}
	parts := strings.Split(domain, ".")
	if len(parts) >= 2 {
		parts = parts[len(parts)-2:]
		return strings.Join(parts, ".")
	} else {
		return ""
	}
}

func (u *URL) FileName() string {
	parts := strings.Split(u.Path, `/`)
	lastPart := parts[len(parts)-1]
	if strings.Contains(lastPart, ".") {
		return lastPart
	} else {
		return ""
	}
}

func (u *URL) FileExt() string {
	fileName := u.FileName()
	if fileName == "" {
		return ""
	}
	parts := strings.Split(fileName, ".")
	return strings.ToLower(parts[len(parts)-1])
}

func (u *URL) ParentPath() string {
	if u.Path == "/" {
		return ""
	} else if strings.HasSuffix(u.Path, "/") {
		if strings.Count(u.Path, "/") == 2 {
			return "/"
		}
		parts := strings.Split(u.Path, "/")
		parts = parts[:len(parts)-2]
		return strings.Join(parts, "/")
	} else {
		if strings.Count(u.Path, "/") == 1 {
			return "/"
		}
		parts := strings.Split(u.Path, "/")
		parts = parts[:len(parts)-1]
		return strings.Join(parts, "/")
	}
}

// InstanceManager manages instances
type InstanceManager struct {
	logger *zap.Logger
}

// NewInstanceManager creates a new InstanceManager
func NewInstanceManager(logger *zap.Logger) *InstanceManager {
	return &InstanceManager{
		logger: logger,
	}
}

// CreateInstance creates a new instance
func (im *InstanceManager) CreateInstance(url string, auth Auth) (*Instance, error) {
	elements := &Elements{
		UsernameSel: "input[name='username']",
		PasswordSel: "input[name='password']",
		SubmitSel:   "button[type='submit']",
	}
	instance := CreateInstance(url, &auth, elements, &DefaultChromeDPContext{})
	return instance, nil
}

// GetInstance retrieves an instance by ID
func (im *InstanceManager) GetInstance(id string) (*Instance, error) {
	instancesLock.Lock()
	defer instancesLock.Unlock()
	instance, ok := instances[id]
	if !ok {
		return nil, errors.New("instance not found")
	}
	return instance, nil
}

// GetInstances retrieves all instances
func (im *InstanceManager) GetInstances() []*Instance {
	instancesLock.Lock()
	defer instancesLock.Unlock()
	instanceList := make([]*Instance, 0, len(instances))
	for _, instance := range instances {
		instanceList = append(instanceList, instance)
	}
	return instanceList
}

// StartInstancesConcurrently starts multiple instances concurrently
func (im *InstanceManager) StartInstancesConcurrently(instanceIDs []string) []error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(instanceIDs))

	for _, id := range instanceIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if err := StartInstance(id); err != nil {
				errChan <- err
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

// StopAllInstances stops all instances
func (im *InstanceManager) StopAllInstances() []error {
	instancesLock.Lock()
	defer instancesLock.Unlock()

	var errors []error
	for id := range instances {
		if err := StopInstance(id); err != nil {
			errors = append(errors, err)
		}
	}

	return errors
}

// StopInstance stops an instance by ID
func (im *InstanceManager) StopInstance(id string) error {
	return StopInstance(id)
}

// DeleteInstance deletes an instance by ID
func (im *InstanceManager) DeleteInstance(id string) error {
	return DeleteInstance(id)
}

// UpdateInstanceStatus updates the status of an instance
func (im *InstanceManager) UpdateInstanceStatus(id string, status string) error {
	instancesLock.Lock()
	instance, ok := instances[id]
	instancesLock.Unlock()
	if !ok {
		return errors.New("instance not found")
	}
	instance.Status = status

	// Update instance status in Redis
	instanceJSON, _ := json.Marshal(instance)
	rdb.HSet(context.Background(), "instances", id, instanceJSON)

	return nil
}

// GetInstanceScreenshot captures a screenshot of an instance
func (im *InstanceManager) GetInstanceScreenshot(id string) ([]byte, error) {
	return DebugInstance(id)
}

func (i *Instance) Execute(action string, params map[string]interface{}) (string, error) {
	// Implement the logic to execute the action on the instance
	// This is a placeholder implementation
	switch action {
	case "exampleAction":
		return "Action executed successfully", nil
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

package actions

import (
	"context"
	"fmt"
	"net/http"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

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

type TokenGen struct {
	cancel context.CancelFunc
	ctx    context.Context
	chrome ChromeDPContext
}

func NewTokenGen(chrome ChromeDPContext) *TokenGen {
	return &TokenGen{chrome: chrome}
}

func (t *TokenGen) StartChrome() (context.Context, error) {
	ctx, cancel := t.chrome.NewContext(context.Background())
	t.cancel = cancel
	t.ctx = ctx
	return ctx, nil
}

func (t *TokenGen) CloseChrome() {
	t.cancel()
}

func (t *TokenGen) GetToken(url, usernameSel, username, passwordSel, password string) (string, error) {
	var cookiesX []*http.Cookie
	if err := t.chrome.Run(t.ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(usernameSel),
		chromedp.SendKeys(usernameSel, username),
		chromedp.Click(passwordSel),
		chromedp.WaitVisible(passwordSel),
		chromedp.SendKeys(passwordSel, password),
		chromedp.Click(`button[type="submit"]`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			cookies, err := network.GetCookies().Do(ctx)
			if err != nil {
				return err
			}
			for _, cookie := range cookies {
				cookiesX = append(cookiesX, &http.Cookie{
					Name:  cookie.Name,
					Value: cookie.Value,
				})
			}
			return nil
		}),
	); err != nil {
		return "", err
	}

	for _, cookie := range cookiesX {
		if cookie.Name == "__Secure-next-auth.session-token" {
			return cookie.Value, nil
		}
	}

	return "", fmt.Errorf("token not found")
}

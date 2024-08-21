package cookie

import (
	"context"
	"errors"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
)

var cookies *[]*network.Cookie = nil

func GetCookies() *[]*network.Cookie {
	return cookies
}

func SetCookies(Cookies []*network.Cookie) {
	cookies = &Cookies
}

func GetChromedpCookies(ctx context.Context) error {
	cook, err := network.GetAllCookies().Do(ctx)
	if err != nil {
		return err
	}

	cookies = &cook

	return nil
}

func SetChromedpCookies(ctx context.Context) error {
	if cookies == nil {
		return errors.New("设置cookies失败")
	}

	err := SetNetWorkCookies(ctx, *cookies)
	if err != nil {
		return err
	}
	return nil
}

func SetNetWorkCookies(ctx context.Context, cookies []*network.Cookie) error {
	for _, cookie := range cookies {
		ok, err := SetNetWorkCookie(ctx, cookie)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("could not set cookie")
		}
	}

	return nil
}

func SetNetWorkCookie(ctx context.Context, cookie *network.Cookie) (bool, error) {
	expr := cdp.TimeSinceEpoch(time.Now().Add(180 * 24 * time.Hour))

	return network.SetCookie(cookie.Name, cookie.Value).
		WithDomain(cookie.Domain).
		WithExpires(&expr).
		WithPath(cookie.Path).
		WithSecure(cookie.Secure).
		WithHTTPOnly(cookie.HTTPOnly).
		WithSameSite(cookie.SameSite).
		Do(ctx)
}

func GetNetWorkCookies(ctx context.Context) ([]*network.Cookie, error) {
	cooks, err := network.GetAllCookies().Do(ctx)
	if err != nil {
		return nil, err
	}

	return cooks, nil
}

func checkField(key string, value interface{}, v *network.Cookie) bool {
	switch key {
	case "Name":
		if value.(string) == v.Name {
			return true
		}
	case "Value":
		if value.(string) == v.Value {
			return true
		}
	case "Domain":
		if value.(string) == v.Domain {
			return true
		}
	case "Path":
		if value.(string) == v.Path {
			return true
		}
	case "Secure":
		if value.(bool) == v.Secure {
			return true
		}
	case "SameSite":
		if value.(network.CookieSameSite) == v.SameSite {
			return true
		}
	case "HTTPOnly":
		if value.(bool) == v.HTTPOnly {
			return true
		}
	case "Expires":
		if value.(float64) == v.Expires {
			return true
		}
	default:
		return false
	}

	return false
}

func CheckCookie(key string, value interface{}) bool {
	if cookies == nil {
		return false
	}
	if len(*cookies) == 0 {
		return false
	}

	for _, v := range *cookies {
		ok := checkField(key, value, v)
		if ok == true {
			return true
		}
	}

	return false
}

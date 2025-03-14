package client

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/gogf/gf/encoding/gurl"
	"github.com/webitel/wlog"
)

type httpClient struct {
	log *wlog.Logger
	cli *http.Client
}

func newHttpClient(log *wlog.Logger) *httpClient {
	cli := http.DefaultClient
	cli.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &httpClient{
		log: log,
		cli: cli,
	}
}

func (c *httpClient) Get(reqUrl string, cookies map[string]string, header map[string]string) (string, int, error) {
	resp, err := c.Request(http.MethodGet, reqUrl, nil, cookies, header)
	if err != nil {
		return "", 0, err
	}

	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}

	body := string(content)
	if resp.StatusCode == http.StatusFound {
		body = resp.Header.Get("Location")
	}

	return body, resp.StatusCode, nil
}
func (c *httpClient) Post(reqUrl string, reqBody io.Reader, cookies map[string]string, header map[string]string) (string, int, error) {
	resp, err := c.Request(http.MethodPost, reqUrl, reqBody, cookies, header)
	if err != nil {
		return "", 0, err
	}

	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}

	body := string(content)
	if resp.StatusCode == http.StatusFound {
		body = resp.Header.Get("Location")
	}

	return body, resp.StatusCode, nil
}

func (c *httpClient) Delete(reqUrl string, cookies map[string]string, header map[string]string) (string, int, error) {
	resp, err := c.Request(http.MethodDelete, reqUrl, nil, cookies, header)
	if err != nil {
		return "", 0, err
	}

	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}

	body := string(content)
	if resp.StatusCode == http.StatusFound {
		body = resp.Header.Get("Location")
	}

	return body, resp.StatusCode, nil
}

func (c *httpClient) Request(method string, reqUrl string, reqBody io.Reader, cookies map[string]string, header map[string]string) (*http.Response, error) {
	u, err := gurl.ParseURL(reqUrl, 2)
	if err != nil {
		return nil, err
	}

	defaultDomain := u["host"]
	req, err := http.NewRequest(method, reqUrl, reqBody)
	if err != nil {
		return nil, err
	}

	// Add common header
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Host", defaultDomain)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 6.3; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/33.0.1750.117 Safari/537.36")
	req.Header.Set("Clientinfo", "os=OSX; osVer=10.15.7; proc=x86; lcid=en-GB; deviceType=1; country=UA; clientName=skype4life; clientVer=1418/8.138.0.203//skype4life; timezone=Europe/Kiev")
	for k, v := range header {
		req.Header.Set(k, v)
	}

	if strings.Index(reqUrl, "ppsecure/post") > -1 {
		// Add other cookies
		maxAge := time.Hour * 24 / time.Second
		if len(cookies) > 0 {
			var newCookies []*http.Cookie
			jar, err := cookiejar.New(nil)
			if err != nil {
				return nil, fmt.Errorf("new cookie jar: %w", err)
			}

			for cK, cV := range cookies {
				newCookies = append(newCookies, &http.Cookie{
					Name:     cK,
					Value:    cV,
					Path:     "/",
					Domain:   defaultDomain,
					MaxAge:   int(maxAge),
					HttpOnly: false,
				})
			}

			jar.SetCookies(req.URL, newCookies)
			c.cli.Jar = jar
		}
	}

	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, err
	}

	c.log.Debug("process http request", wlog.String("url", req.URL.String()), wlog.Any("headers", req.Header),
		wlog.Any("cookies", req.Cookies()), wlog.String("method", req.Method), wlog.Int("status", resp.StatusCode),
		wlog.Any("body", resp.Body), wlog.Any("resp_header", resp.Header))

	return resp, nil
}

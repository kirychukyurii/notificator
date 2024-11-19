package connection

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/encoding/gurl"
)

type client struct {
	cli *http.Client
}

func newClient(timeout time.Duration) *client {
	return &client{
		cli: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *client) Get(reqUrl string, cookies map[string]string, header map[string]string) (string, int, error) {
	resp, err := c.request(http.MethodGet, reqUrl, nil, cookies, header)
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
func (c *client) Post(reqUrl string, reqBody io.Reader, cookies map[string]string, header map[string]string) (string, int, error) {
	resp, err := c.request(http.MethodPost, reqUrl, reqBody, cookies, header)
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

func (c *client) LoginSRF(path string, params url.Values) (string, string, string, error) {
	resp, err := c.request(http.MethodGet, fmt.Sprintf("%s?%s", path, gurl.BuildQuery(params)), nil, nil, nil)
	if err != nil {
		return "", "", "", err
	}

	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", err
	}

	body := string(content)
	if resp.StatusCode == http.StatusFound {
		body = resp.Header.Get("Location")
	}

	var (
		MSPRequ, MSPOK, ppftStr string
	)

	buf := `<input.*?name="PPFT".*?value="(.*?)` + `\"`
	reg := regexp.MustCompile(buf)
	ppfts := reg.FindAllString(body, -1)
	var ppftByte []byte
	if len(ppfts) > 0 {
		for k, v := range ppfts {
			if k == 0 {
				ppftbbf := `value=".*?"`
				ppftreg := regexp.MustCompile(ppftbbf)
				ppftsppft := ppftreg.FindAllString(v, -1)
				ppftByte = []byte(ppftsppft[0])[7:]
				ppftStr = string(ppftByte[0 : len(ppftByte)-1])
			}
		}
	}

	for _, v := range resp.Cookies() {
		if v.Name == "MSPRequ" {
			MSPRequ = v.Value
		}

		if v.Name == "MSPOK" {
			MSPOK = v.Value
		}
	}

	return MSPRequ, MSPOK, ppftStr, nil
}

func (c *client) RegistrationToken(path string, data string, header map[string]string) (string, string, error) {
	resp, err := c.request(http.MethodPost, path, strings.NewReader(data), nil, header)
	if err != nil {
		return "", "", err
	}

	defer resp.Body.Close()

	registrationToken := resp.Header.Get("Set-Registrationtoken")
	location := resp.Header.Get("Location")

	return registrationToken, location, nil
}

func (c *client) request(method string, reqUrl string, reqBody io.Reader, cookies map[string]string, header map[string]string) (*http.Response, error) {
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

	return resp, nil
}

package client

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gogf/gf/encoding/gurl"
)

const (
	ApiLogin string = "https://login.skype.com/login"
)

// liveAuthenticationProvider (legacy) authentication for accounts with Skype usernames and phone numbers.
type liveAuthenticationProvider struct {
	username string
	cli      *httpClient
}

// newLiveAuthenticationProvider creates legacy Live authentication provider
// for accounts with Skype usernames and phone numbers.
func newLiveAuthenticationProvider(cli *httpClient, username string) *liveAuthenticationProvider {
	return &liveAuthenticationProvider{
		cli:      cli,
		username: username,
	}
}

// Auth obtains connection parameters from the Microsoft account login page,
// and performs a login with the given email address or Skype username, and its password.
// This emulates a login to Skype for Web on “login.live.com“.
// Microsoft accounts with two-factor authentication enabled are not supported.
func (l *liveAuthenticationProvider) Auth(password string) (string, string, error) {
	// Start a Microsoft account login from Skype, which will redirect to login.live.com.
	MSPRequ, MSPOK, PPFT, err := l.getParams()
	if MSPOK == "" || MSPRequ == "" || PPFT == "" || err != nil {
		return "", "", fmt.Errorf("get params: one of MSPRequ=%s, MSPOK=%s, PPFT=%s is empty: %w", MSPRequ, MSPOK, PPFT, err)
	}

	fmt.Println("MSPRequ", MSPRequ, "MSPOK", MSPOK, "PPFT", PPFT)

	// Submit the user's credentials.
	opid, t, err := l.submitCredentials(password, MSPRequ, MSPOK, PPFT)
	if err != nil {
		return "", "", fmt.Errorf("submit credentials: %w", err)
	}

	fmt.Println("opid", opid, "t", t)

	if t == "" {
		// Repeat with the 'opid' parameter.
		t, err = l.submitCredentialsOpid(opid, MSPRequ, MSPOK, PPFT)
		if err != nil {
			return "", "", err
		}

		if t == "" {
			return "", "", fmt.Errorf("login: can not find 't' value in Opid response")
		}
	}

	// Now exchange the 't' value for a Skype token.
	token, expires, err := l.token(t)
	if err != nil {
		return "", "", fmt.Errorf("recieve token: %w", err)
	}

	return token, expires, nil
}

// getParams Start a Microsoft account login from Skype,
// which will redirect to login.live.com.
// Collect the value of the cookies MSPRequ, MSPOK and hidden field PPFT.
func (l *liveAuthenticationProvider) getParams() (string, string, string, error) {
	redirectUrl, err := l.redirectURL(ApiLogin)
	if err != nil {
		return "", "", "", err
	}

	MSPRequ, MSPOK, ppftStr, err := l.ppft(redirectUrl)
	if err != nil {
		return "", "", "", err
	}

	return MSPRequ, MSPOK, ppftStr, nil
}

func (l *liveAuthenticationProvider) redirectURL(path string) (string, error) {
	params := url.Values{
		"client_id":    {"578134"},
		"redirect_uri": {"https://web.skype.com"},
	}

	u := fmt.Sprintf("%s/oauth/microsoft?%s", path, gurl.BuildQuery(params))
	redirectUrl, status, err := l.cli.Get(u, nil, nil)
	if err != nil {
		return "", fmt.Errorf("recieve redirect url (%d): %w", status, err)
	}

	fmt.Println("redirect status", status, u)

	if status == http.StatusFound {
		return redirectUrl, nil
	}

	return u, nil
}

func (l *liveAuthenticationProvider) ppft(path string) (string, string, string, error) {
	// follow redirect
	resp, err := l.cli.Request(http.MethodGet, path, nil, nil, nil)
	if err != nil {
		return "", "", "", fmt.Errorf("handle redirect to login.live.com: %w", err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", fmt.Errorf("read response body: %w", err)
	}

	var (
		MSPRequ, MSPOK, ppftStr string
	)

	buf := `<input.*?name="PPFT".*?value="(.*?)` + `\"`
	reg := regexp.MustCompile(buf)
	ppfts := reg.FindAllString(string(body), -1)
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

func (l *liveAuthenticationProvider) submitCredentials(password, msprequ, mspok, ppft string) (string, string, error) {
	// Prepare the Live login page request parameters.
	paramsMap := url.Values{
		"wp":     {"MBI_SSL"},
		"wreply": {"https://lw.skype.com/login/oauth/proxy?client_id=578134&site_name=lw.skype.com&redirect_uri=https%3A%2F%2Fweb.skype.com%2F"},
		"wa":     {"wsignin1.0"},
	}

	cookies := map[string]string{
		"MSPRequ": msprequ,
		"CkTst":   "G" + strconv.Itoa(int(time.Now().UnixNano())/1000000),
		"MSPOK":   mspok,
	}

	var (
		opid, t string
	)

	formData := url.Values{
		"login":        {l.username},
		"passwd":       {password},
		"PPFT":         {ppft},
		"loginoptions": {"3"},
	}

	header := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	reqUrl := fmt.Sprintf("%s?%s", fmt.Sprintf("%s/ppsecure/post.srf", ApiMsacc), gurl.BuildQuery(paramsMap))
	body, _, err := l.cli.Post(reqUrl, strings.NewReader(formData.Encode()), cookies, header)
	if err != nil {
		return "", "", fmt.Errorf("sendCred: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return "", "", fmt.Errorf("new goquery document: %w", err)
	}

	doc.Find("form").Each(func(_ int, s *goquery.Selection) {
		doc.Find("input").Each(func(_ int, s *goquery.Selection) {
			idt, _ := s.Attr("id")
			if idt == "t" {
				t, _ = s.Attr("value")

				return
			}
		})

		if t != "" {
			return
		}

		nameValue, _ := s.Attr("name")
		actionValue, _ := s.Attr("action")
		if nameValue == "fmHF" {
			uslArr := strings.Split(actionValue, "?")
			err = fmt.Errorf("account action required (%s), login with a web browser first", uslArr[0])

			return
		}
	})

	if t != "" {
		return "", t, err
	}

	if err != nil {
		return "", "", err
	}

	r := regexp.MustCompile(`opid=([A-Z0-9]+)`)
	res := r.FindAllStringSubmatch(body, -1)
	if len(res) > 0 {
		if len(res[0]) > 1 {
			opid = res[0][1]
		}
	}

	return opid, t, nil
}

func (l *liveAuthenticationProvider) submitCredentialsOpid(opid, msprequ, mspok, ppft string) (string, error) {
	paramsMap := url.Values{
		"wp":     {"MBI_SSL"},
		"wreply": {"https://lw.skype.com/login/oauth/proxy?client_id=578134&site_name=lw.skype.com&redirect_uri=https%3A%2F%2Fweb.skype.com%2F"},
		"wa":     {"wsignin1.0"},
	}

	cookies := map[string]string{
		"MSPRequ": msprequ,
		"CkTst":   "G" + strconv.Itoa(int(time.Now().UnixNano())/1000000),
		"MSPOK":   mspok,
	}

	formData := url.Values{
		"opid":         {opid},
		"site_name":    {"lw.skype.com"},
		"oauthPartner": {"999"},
		"client_id":    {"578134"},
		"redirect_uri": {"https://web.skype.com"},
		"PPFT":         {ppft},
		"type":         {"28"},
	}

	reqUrl := fmt.Sprintf("%s?%s", fmt.Sprintf("%s/ppsecure/post.srf", ApiMsacc), gurl.BuildQuery(paramsMap))
	header := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	body, status, err := l.cli.Post(reqUrl, strings.NewReader(formData.Encode()), cookies, header)
	if err != nil {
		return "", fmt.Errorf("sendOpid: %w", err)
	}

	fmt.Println("body opid", body, "status", status)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return "", err
	}

	var t string
	doc.Find("input").Each(func(_ int, s *goquery.Selection) {
		idt, _ := s.Attr("id")
		fmt.Println("idt:", idt)
		if idt == "t" {
			t, _ = s.Attr("value")
		}
	})

	return t, nil
}

func (l *liveAuthenticationProvider) token(t string) (string, string, error) {
	paramsMap := url.Values{
		"client_id":    {"578134"},
		"redirect_uri": {"https://web.skype.com"},
	}

	formData := url.Values{
		"t":            {t},
		"client_id":    {"578134"},
		"oauthPartner": {"999"},
		"site_name":    {"lw.skype.com"},
		"redirect_uri": {"https://web.skype.com"},
	}

	header := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	body, _, err := l.cli.Post(fmt.Sprintf("%s/microsoft?%s", ApiLogin, gurl.BuildQuery(paramsMap)), strings.NewReader(formData.Encode()), nil, header)
	if err != nil {
		return "", "", err
	}

	var (
		token, expires string
	)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return "", "", err
	}

	// Find the review items
	doc.Find("input").Each(func(i int, s *goquery.Selection) {
		// For each item found, get the band and title
		attrName, _ := s.Attr("name")
		attrVlue, _ := s.Attr("value")
		if attrName == "skypetoken" {
			token = attrVlue
		}

		if attrName == "expires_in" {
			expires = attrVlue
		}
	})

	if token == "" {
		return "", "", fmt.Errorf("can't get token")
	}

	return token, expires, nil
}

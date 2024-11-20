package connection

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gogf/gf/encoding/gurl"
)

type Session struct {
	Username             string
	Password             string
	SkypeToken           string
	SkypeExpires         string
	RegistrationToken    string
	RegistrationTokenStr string
	RegistrationExpires  string
	LocationHost         string
	EndpointId           string
}

type Profile struct {
	About       string   `json:"about"`
	AvatarUrl   string   `json:"avatarUrl"`
	Birthday    string   `json:"birthday"`
	City        string   `json:"city"`
	Country     string   `json:"country"`
	Emails      []string `json:"emails"`
	FirstName   string   `json:"firstname"`
	Gender      string   `json:"gender"`
	Homepage    string   `json:"homepage"`
	JobTitle    string   `json:"jobtitle"`
	Language    string   `json:"language"`
	LastName    string   `json:"lastname"`
	Mood        string   `json:"mood"`
	PhoneHome   string   `json:"phone_home"`
	PhoneOffice string   `json:"phone_office"`
	Province    string   `json:"province"`
	RichMood    string   `json:"rich_mood"`
	Username    string   `json:"username"` // live:xxxxxxx
}

// TODO: store skype session (token) in session file (as telegram)

type Auth struct {
	loggedIn    bool
	session     *Session
	sessionLock uint32

	profile *Profile
}

// login Skype by web auth
func login(username, password string) (*Auth, error) {
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}

	if password == "" {
		return nil, fmt.Errorf("password is required")
	}

	a := &Auth{
		loggedIn: false,
	}

	// Makes sure that only a single login or Restore can happen at the same time
	if !atomic.CompareAndSwapUint32(&a.sessionLock, 0, 1) {
		return nil, fmt.Errorf("login or restore already running")
	}

	defer atomic.StoreUint32(&a.sessionLock, 0)

	if a.loggedIn {
		username := a.profile.FirstName
		if len(a.profile.LastName) > 0 {
			username = username + a.profile.LastName
		}

		return nil, fmt.Errorf("already logged in as @%s", username)
	}

	getToken := a.AuthLive
	if strings.Index(username, "@") > -1 {
		getToken = a.SOAP
	}

	if err := getToken(username, password); err != nil {
		return nil, err
	}

	a.session.LocationHost = API_MSGSHOST
	if err := a.SkypeRegistrationTokenProvider(a.session.SkypeToken); err != nil {
		return nil, fmt.Errorf("SkypeRegistrationTokenProvider: %w", err)
	}

	a.session.Username = username
	a.session.Password = password

	if err := a.GetUserId(a.session.SkypeToken); err != nil {
		return nil, fmt.Errorf("GetUserId: %w", err)
	}

	return a, nil
}

func (a *Auth) IsLoginInProgress() bool {
	return a.sessionLock == 1
}

// AuthLive (legacy) authentication for accounts with Skype usernames and phone numbers.
func (a *Auth) AuthLive(username, password string) error {
	// This will redirect to login.live.com. Collect the value of the hidden field named PPFT.
	MSPRequ, MSPOK, PPFT, err := a.getParams()
	if MSPOK == "" || MSPRequ == "" || PPFT == "" || err != nil {
		return fmt.Errorf("get params: one of MSPRequ, MSPOK, PPFT is empty: %w", err)
	}

	// Send username and password
	paramsMap := url.Values{}
	paramsMap.Set("wp", "MBI_SSL")
	paramsMap.Set("wreply", "https://lw.skype.com/login/oauth/proxy?client_id=578134&site_name=lw.skype.com&redirect_uri=https%3A%2F%2Fweb.skype.com%2F")
	paramsMap.Set("wa", "wsignin1.0")

	cookies := map[string]string{
		"MSPRequ": MSPRequ,
		"CkTst":   "G" + strconv.Itoa(int(time.Now().UnixNano())/1000000),
		"MSPOK":   MSPOK,
	}

	opid, t, err := a.sendCred(paramsMap, username, password, PPFT, cookies)
	if err != nil {
		return err
	}

	if t == "" {
		cookies["CkTst"] = strconv.Itoa(int(time.Now().UnixNano() / 1000000))
		t, err = a.sendOpid(paramsMap, PPFT, opid, cookies)
		if err != nil {
			return err
		}

		if t == "" {
			return fmt.Errorf("login: can not find 't' value in Opid response")
		}
	}

	// Get token and RegistrationExpires
	if err = a.getToken(t); err != nil {
		return fmt.Errorf("recieve token: %w", err)
	}

	return nil
}

// SOAP authentication for Microsoft accounts with an email address as the username.
// Microsoft accounts with two-factor authentication enabled are supported if an application-specific password
// is provided.
// Skype accounts must be linked to a Microsoft account with an email address, otherwise SkypeAuthErr.
// See the exception definitions for other possible causes.
func (a *Auth) SOAP(username, password string) error {
	type RequestedSecurityToken struct {
		BinarySecurityToken string `xml:"BinarySecurityToken"`
	}

	type RequestSecurityTokenResponse struct {
		TokenType string                 `xml:"TokenType"`
		AppliesTo string                 `xml:"AppliesTo"`
		LifeTime  string                 `xml:"LifeTime"`
		ReSeToken RequestedSecurityToken `xml:"RequestedSecurityToken"`
	}

	type RequestSecurityTokenResponseCollection struct {
		Response RequestSecurityTokenResponse `xml:"RequestSecurityTokenResponse"`
	}

	type EnvelopeBody struct {
		Collection RequestSecurityTokenResponseCollection `xml:"RequestSecurityTokenResponseCollection"`
	}

	type EnvelopeFault struct {
		FaultCode   string `xml:"faultcode"`
		FaultString string `xml:"faultstring"`
	}

	type EnvelopeXML struct {
		XMLName xml.Name      `xml:"Envelope"`
		Header  string        `xml:"Header"`
		Body    EnvelopeBody  `xml:"Body"`
		Fault   EnvelopeFault `xml:"Fault"`
	}

	type EdgeResp struct {
		SkypeToken string `json:"skypetoken"`
		ExpiresIn  int32  `json:"expiresIn"`
		SkypeId    string `json:"skypeid"`
		SignInName string `json:"signinname"`
		Anid       string `json:"anid"`
		Status     struct {
			Code int32  `json:"code"`
			Text string `json:"text"`
		} `json:"status"`
	}

	// An authentication provider that connects via Microsoft account SOAP authentication.
	template := `
    <Envelope xmlns='http://schemas.xmlsoap.org/soap/envelope/'
       xmlns:wsse='http://schemas.xmlsoap.org/ws/2003/06/secext'
       xmlns:wsp='http://schemas.xmlsoap.org/ws/2002/12/policy'
       xmlns:wsa='http://schemas.xmlsoap.org/ws/2004/03/addressing'
       xmlns:wst='http://schemas.xmlsoap.org/ws/2004/04/trust'
       xmlns:ps='http://schemas.microsoft.com/Passport/SoapServices/PPCRL'>
       <Header>
           <wsse:Security>
               <wsse:UsernameToken Id='user'>
                   <wsse:Username>%s</wsse:Username>
                   <wsse:Password>%s</wsse:Password>
               </wsse:UsernameToken>
           </wsse:Security>
       </Header>
       <Body>
           <ps:RequestMultipleSecurityTokens Id='RSTS'>
               <wst:RequestSecurityToken Id='RST0'>
                   <wst:RequestType>http://schemas.xmlsoap.org/ws/2004/04/security/trust/Issue</wst:RequestType>
                   <wsp:AppliesTo>
                       <wsa:EndpointReference>
                           <wsa:Address>wl.skype.com</wsa:Address>
                       </wsa:EndpointReference>
                   </wsp:AppliesTo>
                   <wsse:PolicyReference URI='MBI_SSL'></wsse:PolicyReference>
               </wst:RequestSecurityToken>
           </ps:RequestMultipleSecurityTokens>
       </Body>
    </Envelope>`
	data := fmt.Sprintf(template, replaceSymbol(username), replaceSymbol(password))

	req := newClient(30 * time.Second)
	body, _, err := req.Post(fmt.Sprintf("%s/RST.srf", API_MSACC), strings.NewReader(data), nil, nil)
	if err != nil {
		return fmt.Errorf("couldn't retrieve security token from login response: %w", err)
	}

	var envelopeResult EnvelopeXML
	if err = xml.Unmarshal([]byte(body), &envelopeResult); err != nil {
		return fmt.Errorf("get token err: parse EnvelopeXML err: %w", err)
	}

	if envelopeResult.Body.Collection.Response.ReSeToken.BinarySecurityToken == "" {
		if envelopeResult.Fault.FaultCode == "wsse:FailedAuthentication" {
			return fmt.Errorf("please confirm that your account password is entered correctly")
		}

		return fmt.Errorf("get token err: can not find BinarySecurityToken")
	}

	data2 := map[string]interface{}{
		"partner":      999,
		"access_token": envelopeResult.Body.Collection.Response.ReSeToken.BinarySecurityToken,
		"scopes":       "client",
	}

	params, _ := json.Marshal(data2)
	body, _, err = req.Post(API_EDGE, strings.NewReader(string(params)), nil, nil)
	if err != nil {
		return fmt.Errorf("get token err: exchangeToken: %w", err)
	}

	edgeResp := EdgeResp{}
	if err := json.Unmarshal([]byte(body), &edgeResp); err != nil {
		return err
	}

	if edgeResp.SkypeToken == "" || edgeResp.ExpiresIn == 0 {
		return fmt.Errorf("err status code: %s, status text: %s", strconv.FormatInt(int64(edgeResp.Status.Code), 10), edgeResp.Status.Text)
	}

	a.session = &Session{
		SkypeToken:   edgeResp.SkypeToken,
		SkypeExpires: strconv.FormatInt(int64(edgeResp.ExpiresIn), 10),
	}

	return nil
}

// SkypeRegistrationTokenProvider Request a new registration token using a current Skype token.
// Args:
//
//	skypeToken (str): existing Skype token
//
// Returns:
//
//	(str, datetime.datetime, str, SkypeEndpoint) tuple: registration token, associated expiry if known,
//														resulting endpoint hostname, endpoint if provided
//
// Raises:
//
//	.SkypeAuthException: if the login request is rejected
//	.SkypeApiException: if the login form can't be processed
//
// * Value used for the `ConnInfo` header of the request for the registration token.
func (a *Auth) SkypeRegistrationTokenProvider(skypeToken string) error {
	if skypeToken == "" {
		return fmt.Errorf("skype token not exist")
	}

	secs := strconv.Itoa(int(time.Now().Unix()))
	lockAndKeyResponse := getMac256Hash(secs)
	LockAndKey := "appId=" + SKYPEWEB_LOCKANDKEY_APPID + "; time=" + secs + "; lockAndKeyResponse=" + lockAndKeyResponse
	req := newClient(30 * time.Second)
	header := map[string]string{
		"Authentication":   "skypetoken=" + skypeToken,
		"LockAndKey":       LockAndKey,
		"BehaviorOverride": "redirectAs404",
	}

	data := map[string]interface{}{
		"endpointFeatures": "Agent",
	}

	params, _ := json.Marshal(data)
	registrationTokenStr, location, err := req.RegistrationToken(a.session.LocationHost+"/v1/users/"+DEFAULT_USER+"/endpoints", string(params), header)
	if err != nil {
		return err
	}

	if len(location) < 1 {
		return fmt.Errorf("didn't get enpoint location")
	}

	locationArr := strings.Split(location, "/v1")
	a.storeInfo(registrationTokenStr, a.session.LocationHost)
	if locationArr[0] != a.session.LocationHost {
		newRegistrationToken, _, err := req.RegistrationToken(location, string(params), header)
		if err != nil {
			return fmt.Errorf("HttpPostRegistrationToken: %w", err)
		}

		a.storeInfo(newRegistrationToken, locationArr[0])
	}

	return nil
}

func (a *Auth) GetUserId(skypetoken string) error {
	req := newClient(30 * time.Second)
	headers := map[string]string{
		"x-skypetoken": skypetoken,
	}

	body, _, err := req.Get(fmt.Sprintf("%s/users/self/profile", API_USER), nil, headers)
	if err != nil {
		return fmt.Errorf("get userId: %w", err)
	}

	userProfile := Profile{}
	if err := json.Unmarshal([]byte(body), &userProfile); err != nil {
		return err
	}

	a.profile = &userProfile

	return nil
}

// getParams This will redirect to login.live.com. Collect the value of the hidden field named PPFT.
func (a *Auth) getParams() (MSPRequ, MSPOK, PPFT string, err error) {
	params := url.Values{}
	params.Set("client_id", "578134")
	params.Set("redirect_uri", "https://web.skype.com")
	req := newClient(30 * time.Second)

	redirectUrl, _, err := req.Get(fmt.Sprintf("%s/oauth/microsoft?%s", API_LOGIN, gurl.BuildQuery(params)), nil, nil)
	if err != nil {
		return "", "", "", fmt.Errorf("recieve redirect url: %w", err)
	}

	loginSpfParam := url.Values{}
	MSPRequ, MSPOK, ppftStr, err := req.LoginSRF(redirectUrl, loginSpfParam)
	if err != nil {
		return "", "", "", fmt.Errorf("recieve loginSpf: %w", err)
	}

	return MSPRequ, MSPOK, ppftStr, nil
}

func (a *Auth) sendCred(paramsMap url.Values, username, password, PPFT string, cookies map[string]string) (string, string, error) {
	var (
		opid, t string
	)

	req := newClient(30 * time.Second)
	formData := url.Values{
		"login":        {username},
		"passwd":       {password},
		"PPFT":         {PPFT},
		"loginoptions": {"3"},
	}

	header := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	reqUrl := fmt.Sprintf("%s?%s", fmt.Sprintf("%s/ppsecure/post.srf", API_MSACC), gurl.BuildQuery(paramsMap))
	body, _, err := req.Post(reqUrl, strings.NewReader(formData.Encode()), cookies, header)
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
	res := find(body, r)
	if len(res) > 0 {
		if len(res[0]) > 1 {
			opid = res[0][1]
		}
	}

	return opid, t, nil
}

func (a *Auth) sendOpid(paramsMap url.Values, PPFT, opid string, cookies map[string]string) (string, error) {
	req := newClient(30 * time.Second)
	formData := url.Values{
		"opid":         {opid},
		"site_name":    {"lw.skype.com"},
		"oauthPartner": {"999"},
		"client_id":    {"578134"},
		"redirect_uri": {"https://web.skype.com"},
		"PPFT":         {PPFT},
		"type":         {"28"},
	}

	reqUrl := fmt.Sprintf("%s?%s", fmt.Sprintf("%s/ppsecure/post.srf", API_MSACC), gurl.BuildQuery(paramsMap))
	header := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	body, _, err := req.Post(reqUrl, strings.NewReader(formData.Encode()), cookies, header)
	if err != nil {
		return "", fmt.Errorf("sendOpid: %w", err)
	}

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

func (a *Auth) getToken(t string) error {
	// Now pass the login credentials over
	paramsMap := url.Values{}
	paramsMap.Set("client_id", "578134")
	paramsMap.Set("redirect_uri", "https://web.skype.com")

	req := newClient(30 * time.Second)
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

	body, _, err := req.Post(fmt.Sprintf("%s/microsoft?%s", API_LOGIN, gurl.BuildQuery(paramsMap)), strings.NewReader(formData.Encode()), nil, header)
	if err != nil {
		return err
	}

	var (
		token, expires string
	)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return err
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

	a.session = &Session{
		SkypeToken:   token,
		SkypeExpires: expires,
	}

	if token == "" {
		return fmt.Errorf("can't get token")
	}

	return nil
}

func (a *Auth) storeInfo(registrationTokenStr string, locationHost string) {
	regArr := strings.Split(registrationTokenStr, ";")
	registrationToken := ""
	registrationExpires := ""
	if len(regArr) > 0 {
		for _, v := range regArr {
			v = strings.Replace(v, " ", "", -1)
			if len(v) > 0 {
				if strings.Index(v, "registrationToken=") > -1 {
					vv := strings.Split(v, "registrationToken=")
					registrationToken = vv[1]
				} else {
					vv := strings.Split(v, "=")
					if vv[0] == "expires" {
						registrationExpires = vv[1]
					}

					if vv[0] == "endpointId" {
						if vv[1] != "" {
							a.session.EndpointId = vv[1]
						}
					}
				}
			}
		}
	}

	a.session.LocationHost = locationHost
	a.session.RegistrationToken = registrationToken
	a.session.RegistrationExpires = registrationExpires
	a.session.RegistrationTokenStr = registrationTokenStr
	a.loggedIn = true

	if strings.Index(registrationTokenStr, "endpointId=") == -1 {
		registrationTokenStr = registrationTokenStr + "; endpointId=" + a.session.EndpointId
	}

	return
}

// getMac256Hash generates SKYPEWEB_LOCKANDKEY_SECRET
func getMac256Hash(secs string) string {
	clearText := secs + SKYPEWEB_LOCKANDKEY_APPID
	zeroNum := (8 - len(clearText)%8)
	for i := 0; i < zeroNum; i++ {
		clearText += "0"
	}

	cchClearText := len(clearText) / 4
	pClearText := make([]int, cchClearText)
	for i := 0; i < cchClearText; i++ {
		mib := 0
		for pos := 0; pos < 4; pos++ {
			len1 := 4*i + pos
			b := int([]rune(clearText[len1 : len1+1])[0])
			mi := int(math.Pow(256, float64(pos)))
			mib += mi * b
		}

		pClearText[i] = mib
	}

	sha256Hash := []int{
		0, 0, 0, 0,
	}

	screactKeyStr := secs + SKYPEWEB_LOCKANDKEY_SECRET
	h := sha256.New()
	h.Write([]byte(screactKeyStr))
	sum := h.Sum(nil)
	hash_str := strings.ToUpper(hex.EncodeToString(sum))
	sha256len := len(sha256Hash)
	for s := 0; s < sha256len; s++ {
		sha256Hash[s] = 0
		for pos := 0; pos < 4; pos++ {
			dpos := 8*s + pos*2
			mi1 := int(math.Pow(256, float64(pos)))
			inthash := hash_str[dpos : dpos+2]
			inthash1, _ := strconv.ParseInt(inthash, 16, 64)
			sha256Hash[s] += int(inthash1) * mi1
		}
	}

	qwMAC, qwSum := cs64(pClearText, sha256Hash)
	macParts := []int{
		qwMAC,
		qwSum,
		qwMAC,
		qwSum,
	}

	scans := []int{0, 0, 0, 0}
	for i, sha := range sha256Hash {
		scans[i] = int64Xor(sha, macParts[i])
	}

	hexString := ""
	for _, scan := range scans {
		hexString += int32ToHexString(scan)
	}

	return hexString
}

func int32ToHexString(n int) (hexString string) {
	hexChars := "0123456789abcdef"
	for i := 0; i < 4; i++ {
		num1 := (n >> (i*8 + 4)) & 15
		num2 := (n >> (i * 8)) & 15
		hexString += hexChars[num1 : num1+1]
		hexString += hexChars[num2 : num2+1]
	}

	return
}

func int64Xor(a int, b int) (sc int) {
	sA := fmt.Sprintf("%b", a)
	sB := fmt.Sprintf("%b", b)
	sC := ""
	sD := ""
	diff := math.Abs(float64(len(sA) - len(sB)))

	for d := 0; d < int(diff); d++ {
		sD += "0"
	}

	if len(sA) < len(sB) {
		sD += sA
		sA = sD
	} else if len(sB) < len(sA) {
		sD += sB
		sB = sD
	}

	for a := 0; a < len(sA); a++ {
		if sA[a] == sB[a] {
			sC += "0"
		} else {
			sC += "1"
		}
	}

	to2, _ := strconv.ParseInt(sC, 2, 64)
	xor, _ := strconv.Atoi(fmt.Sprintf("%d", to2))

	return xor
}

func cs64(pdwData, pInHash []int) (qwMAC int, qwSum int) {
	MODULUS := 2147483647
	CS64_a := pInHash[0] & MODULUS
	CS64_b := pInHash[1] & MODULUS
	CS64_c := pInHash[2] & MODULUS
	CS64_d := pInHash[3] & MODULUS
	CS64_e := 242854337
	pos := 0
	qwDatum := 0
	qwMAC = 0
	qwSum = 0
	pdwLen := len(pdwData) / 2

	for i := 0; i < pdwLen; i++ {
		qwDatum = int(pdwData[pos])
		pos += 1
		qwDatum *= CS64_e
		qwDatum = qwDatum % MODULUS
		qwMAC += qwDatum
		qwMAC *= CS64_a
		qwMAC += CS64_b
		qwMAC = qwMAC % MODULUS
		qwSum += qwMAC
		qwMAC += int(pdwData[pos])
		pos += 1
		qwMAC *= CS64_c
		qwMAC += CS64_d
		qwMAC = qwMAC % MODULUS
		qwSum += qwMAC
	}

	qwMAC += CS64_b
	qwMAC = qwMAC % MODULUS
	qwSum += CS64_d
	qwSum = qwSum % MODULUS

	return qwMAC, qwSum
}

func find(htm string, re *regexp.Regexp) [][]string {
	return re.FindAllStringSubmatch(htm, -1)
}

func replaceSymbol(str string) string {
	str = strings.ReplaceAll(str, "&", "&amp;")
	str = strings.ReplaceAll(str, "<", "&lt;")
	str = strings.ReplaceAll(str, ">", "&gt;")

	return str
}

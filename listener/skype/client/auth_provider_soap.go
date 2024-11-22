package client

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

const (
	ApiEdge string = "https://edge.skype.com/rps/v1/rps/skypetoken"
)

var authTemplate = `
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

// soapAuthenticationProvider connects via Microsoft account SOAP authentication.
type soapAuthenticationProvider struct {
	username string
	cli      *httpClient
}

// newSOAPAuthenticationProvider creates SOAP authentication provider
// that connects via Microsoft account SOAP authentication.
func newSOAPAuthenticationProvider(cli *httpClient, username string) *soapAuthenticationProvider {
	return &soapAuthenticationProvider{
		username: username,
		cli:      cli,
	}
}

// Auth perform a SOAP login with the given email address or Skype username, and its password.
// Microsoft accounts with two-factor authentication enabled must provide an application-specific password.
func (s *soapAuthenticationProvider) Auth(password string) (string, string, error) {
	bst, err := s.secToken(password)
	if err != nil {
		return "", "", err
	}

	token, expires, err := s.exchangeToken(bst)
	if err != nil {
		return "", "", fmt.Errorf("exchange token err: %w", err)
	}

	return token, expires, nil
}

func (s *soapAuthenticationProvider) secToken(password string) (string, error) {
	data := fmt.Sprintf(authTemplate, replaceSymbol(s.username), replaceSymbol(password))
	body, _, err := s.cli.Post(fmt.Sprintf("%s/RST.srf", ApiMsacc), strings.NewReader(data), nil, nil)
	if err != nil {
		return "", fmt.Errorf("couldn't retrieve security token from login response: %w", err)
	}

	var envelopeResult EnvelopeXML
	if err = xml.Unmarshal([]byte(body), &envelopeResult); err != nil {
		return "", fmt.Errorf("get token err: parse EnvelopeXML err: %w", err)
	}

	if envelopeResult.Body.Collection.Response.ReSeToken.BinarySecurityToken == "" {
		if envelopeResult.Fault.FaultCode == "wsse:FailedAuthentication" {
			return "", fmt.Errorf("please confirm that your account password is entered correctly")
		}

		return "", fmt.Errorf("get token err: can not find BinarySecurityToken")
	}

	return envelopeResult.Body.Collection.Response.ReSeToken.BinarySecurityToken, nil
}

func (s *soapAuthenticationProvider) exchangeToken(bst string) (string, string, error) {
	data2 := map[string]interface{}{
		"partner":      999,
		"access_token": bst,
		"scopes":       "client",
	}

	params, err := json.Marshal(data2)
	if err != nil {
		return "", "", err
	}
	body, _, err := s.cli.Post(ApiEdge, strings.NewReader(string(params)), nil, nil)
	if err != nil {
		return "", "", fmt.Errorf("get token err: exchangeToken: %w", err)
	}

	edgeResp := EdgeResp{}
	if err := json.Unmarshal([]byte(body), &edgeResp); err != nil {
		return "", "", err
	}

	if edgeResp.SkypeToken == "" || edgeResp.ExpiresIn == 0 {
		return "", "", fmt.Errorf("err status code: %s, status text: %s", strconv.FormatInt(int64(edgeResp.Status.Code), 10), edgeResp.Status.Text)
	}

	return edgeResp.SkypeToken, strconv.FormatInt(int64(edgeResp.ExpiresIn), 10), nil
}

func replaceSymbol(str string) string {
	str = strings.ReplaceAll(str, "&", "&amp;")
	str = strings.ReplaceAll(str, "<", "&lt;")
	str = strings.ReplaceAll(str, ">", "&gt;")

	return str
}

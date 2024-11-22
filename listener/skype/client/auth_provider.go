package client

import "strings"

// authenticationProvider that connects via Microsoft account authentication.
type authenticationProvider interface {
	// Auth getting Skype token and associated expiry if known
	Auth(password string) (string, string, error)
}

// newEndpoint creates authenticationProvider, that can handle in two ways:
//   - liveAuthenticationProvider - for accounts with Skype usernames and phone numbers (requires
//     calling out to the MS OAuth page, and retrieving the Skype token);
//   - soapAuthenticationProvider - performs SOAP login (authentication with a Microsoft account
//     email address and password (or application-specific token),
//     using an endpoint to obtain a security token, and exchanging that for a Skype token).
func newAuthenticationProvider(cli *httpClient, username string) authenticationProvider {
	if strings.Index(username, "@") > -1 {
		return newSOAPAuthenticationProvider(cli, username)
	}

	return newLiveAuthenticationProvider(cli, username)
}

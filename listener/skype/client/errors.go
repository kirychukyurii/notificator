package client

import "errors"

var (
	SkypeRateLimitErr = errors.New("rate limit has been reached: cooldown on authentication, message sending, or another action performed too frequently")

	// SkypeAuthErr reasons why a login may be rejected, including but not limited to:
	//  - an incorrect username or password
	//  - two-factor authentication
	//  - rate-limiting after multiple failed login attempts
	//  - a captcha being required
	//  - an update to the Terms of Service that must be accepted
	SkypeAuthErr = errors.New("authentication can not be completed")
)

package client

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	SkypewebLockandkeyAppid  = "msmsgs@msnmsgr.com"
	SkypewebLockandkeySecret = "Q1P7W2E4J9R8U3S5"
)

// registrationToken authorize on endpoint and request a new registration
// token using a current Skype token.
func (e *endpoint) registrationToken() error {
	secs := strconv.Itoa(int(time.Now().Unix()))
	lockAndKeyResponse := getMac256Hash(secs)
	LockAndKey := "appId=" + SkypewebLockandkeyAppid + "; time=" + secs + "; lockAndKeyResponse=" + lockAndKeyResponse
	header := map[string]string{
		"Authentication":   "skypetoken=" + e.skypeToken,
		"LockAndKey":       LockAndKey,
		"BehaviorOverride": "redirectAs404",
	}

	data := map[string]interface{}{
		"endpointFeatures": "Agent",
	}

	params, err := json.Marshal(data)
	if err != nil {
		return err
	}

	registrationTokenStr, location, _, err := e.registrationTokenWithLocation(http.MethodPost, fmt.Sprintf("%s/v1/users/ME/endpoints", e.msgsHost), string(params), header)
	if err != nil {
		return err
	}

	if len(location) < 1 {
		return fmt.Errorf("didn't get endpoint location")
	}

	locationArr := strings.Split(location, "/v1")
	e.registrationTokenProps(registrationTokenStr)

	// Skype is requiring the use of a different hostname.
	if locationArr[0] != e.msgsHost {
		// Don't accept the token if present, we need to re-register first.
		newRegistrationToken, _, status, err := e.registrationTokenWithLocation(http.MethodPost, location, string(params), header)
		if err != nil {
			return fmt.Errorf("HttpPostRegistrationToken: %w", err)
		}

		if status == http.StatusMethodNotAllowed {
			newRegistrationToken, _, status, err = e.registrationTokenWithLocation(http.MethodPut, location, string(params), header)
			if err != nil {
				return fmt.Errorf("HttpPostRegistrationToken: %w", err)
			}
		}

		if status != http.StatusCreated && status != http.StatusOK {
			return fmt.Errorf("HttpPostRegistrationToken: status %d", status)
		}

		e.registrationTokenProps(newRegistrationToken)
		e.msgsHost = locationArr[0]
	}

	return nil
}

func (e *endpoint) registrationTokenProps(registrationTokenStr string) {
	var regToken, regTokenExpires string
	regArr := strings.Split(registrationTokenStr, ";")
	if len(regArr) > 0 {
		for _, v := range regArr {
			v = strings.Replace(v, " ", "", -1)
			if len(v) > 0 {
				if strings.Index(v, "registrationToken=") > -1 {
					vv := strings.Split(v, "registrationToken=")
					regToken = vv[1]
				} else {
					vv := strings.Split(v, "=")
					if vv[0] == "expires" {
						if exp, err := strconv.Atoi(vv[1]); err == nil {
							regTokenExpires = strconv.Itoa(exp - int(time.Now().Unix()))
						}
					}

					if vv[0] == "endpointId" {
						if vv[1] != "" {
							e.id = vv[1]
						}
					}
				}
			}
		}
	}

	e.token = regToken
	e.expires = regTokenExpires
	if strings.Index(registrationTokenStr, "endpointId=") == -1 {
		registrationTokenStr = registrationTokenStr + "; endpointId=" + e.id
	} else {
		e.tokenProps = registrationTokenStr
	}
}

func (e *endpoint) registrationTokenWithLocation(method, path, body string, headers map[string]string) (string, string, int, error) {
	resp, err := e.cli.Request(method, path, strings.NewReader(body), nil, headers)
	if err != nil {
		return "", "", 0, err
	}

	defer resp.Body.Close()

	return resp.Header.Get("Set-Registrationtoken"), resp.Header.Get("Location"), resp.StatusCode, nil
}

// getMac256Hash generates the lock-and-key response, needed to acquire registration tokens.
func getMac256Hash(secs string) string {
	clearText := secs + SkypewebLockandkeyAppid
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

	screactKeyStr := secs + SkypewebLockandkeySecret
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

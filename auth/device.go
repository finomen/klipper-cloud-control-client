package auth

import (
	"encoding/json"
	"fmt"
	"klipper-cloud-control-client/config"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"
)

const (
	codePath       = "/auth/get_code"
	tokenPath      = "/auth/get_token"
	checkTokenPath = "/auth/check_token"
)

type DeviceAuth struct {
}

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationUrl string `json:"verification_url"`
	ExpiresIn       int64  `json:"expires_in"`
	Interval        int64  `json:"interval"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	IdToken      string `json:"id_token"`
}

type tokenError struct {
	Error string `json:"error"`
}

type CodeRequest struct {
	deviceCode deviceCodeResponse
}

func (cr CodeRequest) GetUrl() string {
	return cr.deviceCode.VerificationUrl
}

func (cr CodeRequest) GetCode() string {
	return cr.deviceCode.UserCode
}

func (cr CodeRequest) GetToken() (*config.Token, error) {
	checker := time.NewTicker(time.Second * time.Duration(cr.deviceCode.Interval))
	timeout := time.NewTicker(time.Second * time.Duration(cr.deviceCode.ExpiresIn))

	for {
		select {
		case _ = <-checker.C:
			result, err := http.PostForm(fmt.Sprintf("%s%s", config.GetConfig().GetHostname(), tokenPath), url.Values{
				"device_code": []string{cr.deviceCode.DeviceCode},
				"grant_type":  []string{"http://oauth.net/grant_type/device/1.0"},
			})
			if err != nil {
				return nil, err
			}

			body := make([]byte, result.ContentLength)
			result.Body.Read(body)

			if result.StatusCode == 200 {
				resp := tokenResponse{}

				if err := json.Unmarshal(body, &resp); err != nil {
					fmt.Println("Failed to parse token: ", err)
					continue
				}

				fmt.Println("Authorized")
				return &config.Token{
					AccessToken:  resp.AccessToken,
					TokenType:    resp.TokenType,
					ExpiresAt:    time.Now().Add(time.Second * time.Duration(resp.ExpiresIn)),
					RefreshToken: resp.RefreshToken,
					IdToken:      resp.IdToken,
				}, nil
			}

			resp := tokenError{}

			if err := json.Unmarshal(body, &resp); err != nil {
				fmt.Println("Failed to parse token response: ", err)
				continue
			}

			switch resp.Error {
			case "authorization_pending":
				continue
			case "slow_down":
				fmt.Println("Got slow down error")
				continue
			default:
				return nil, fmt.Errorf("Get token failed: %s", resp.Error)
			}

		case _ = <-timeout.C:
			return nil, fmt.Errorf("Authentication timed out")
		}
	}

}

func (auth DeviceAuth) GetDeviceCode() (*CodeRequest, error) {
	result, err := http.Get(fmt.Sprintf("%s%s", config.GetConfig().GetHostname(), codePath))
	if err != nil {
		return nil, err
	}
	if result.StatusCode != 200 {
		return nil, fmt.Errorf("Failed to get code: %s (%s)", result.StatusCode, result.Status)
	}

	body := make([]byte, result.ContentLength)
	result.Body.Read(body)

	resp := deviceCodeResponse{}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &CodeRequest{
		deviceCode: resp,
	}, nil

}

func RefreshToken(token config.Token) (*config.Token, error) {
	//FIXME: only if needed
	result, err := http.PostForm(fmt.Sprintf("%s%s", config.GetConfig().GetHostname(), tokenPath), url.Values{
		"refresh_token": []string{token.RefreshToken},
		"grant_type":    []string{"refresh_token"},
	})
	if err != nil {
		return nil, err
	}

	body := make([]byte, result.ContentLength)
	result.Body.Read(body)

	if result.StatusCode == 200 {
		resp := tokenResponse{}

		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, err
		}

		fmt.Println("Authorized")
		return &config.Token{
			AccessToken:  resp.AccessToken,
			TokenType:    resp.TokenType,
			ExpiresAt:    time.Now().Add(time.Second * time.Duration(resp.ExpiresIn)),
			RefreshToken: token.RefreshToken,
			IdToken:      resp.IdToken,
		}, nil
	}

	resp := tokenError{}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("Failed to parse token response: %s", err)
	}

	return nil, fmt.Errorf("Failed to refresh token: %s", resp.Error)

}

func DoAuth(token *config.Token) (*cookiejar.Jar, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	client := http.Client{
		Jar: jar,
	}

	result, err := client.PostForm(fmt.Sprintf("%s%s", config.GetConfig().GetHostname(), checkTokenPath), url.Values{
		"google_token": []string{token.IdToken},
	})
	if err != nil {
		return nil, err
	}

	if result.StatusCode == 200 {
		return jar, nil
	} else {
		return nil, fmt.Errorf("Auth failed")
	}

}

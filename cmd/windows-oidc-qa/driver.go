//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"
)

type liveQACasdoorDriver struct {
	username string
	password string
	client   *http.Client
	calls    atomic.Int32
}

func (d *liveQACasdoorDriver) Open(authorizeURL string) error {
	d.calls.Add(1)
	target, err := url.Parse(authorizeURL)
	if err != nil || target.Scheme != "https" || target.Host == "" {
		return errors.New("Casdoor authorization URL is unsafe")
	}
	query := target.Query()
	if query.Get("response_type") != "code" || query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" || query.Get("state") == "" || query.Get("nonce") == "" {
		return errors.New("Casdoor authorization request omitted PKCE, state, or nonce")
	}
	redirect, err := url.Parse(query.Get("redirect_uri"))
	if err != nil || redirect.Scheme != "http" || redirect.Hostname() != "127.0.0.1" || redirect.Path != "/callback" {
		return errors.New("Casdoor authorization request used an unsafe callback")
	}
	apiQuery := url.Values{
		"clientId":              {query.Get("client_id")},
		"responseType":          {query.Get("response_type")},
		"redirectUri":           {query.Get("redirect_uri")},
		"type":                  {"code"},
		"scope":                 {query.Get("scope")},
		"state":                 {query.Get("state")},
		"nonce":                 {query.Get("nonce")},
		"code_challenge_method": {query.Get("code_challenge_method")},
		"code_challenge":        {query.Get("code_challenge")},
	}
	var application struct {
		Status string `json:"status"`
		Data   struct {
			Name         string `json:"name"`
			Organization string `json:"organization"`
		} `json:"data"`
	}
	if err := liveQAJSONRequest(d.client, http.MethodGet, target.Scheme+"://"+target.Host+"/api/get-app-login?"+apiQuery.Encode(), nil, &application); err != nil {
		return err
	}
	if application.Status != "ok" || application.Data.Name == "" || application.Data.Organization == "" {
		return errors.New("Casdoor application login configuration is unavailable")
	}
	input := map[string]any{
		"application": application.Data.Name, "organization": application.Data.Organization,
		"username": d.username, "password": d.password, "autoSignin": false,
		"signinMethod": "Password", "type": "code", "language": "en",
	}
	var login struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	if err := liveQAJSONRequest(d.client, http.MethodPost, target.Scheme+"://"+target.Host+"/api/login?"+apiQuery.Encode(), input, &login); err != nil {
		return err
	}
	var code string
	if login.Status != "ok" || json.Unmarshal(login.Data, &code) != nil || len(code) < 16 {
		return errors.New("Casdoor employee login was rejected")
	}
	callbackQuery := redirect.Query()
	callbackQuery.Set("code", code)
	callbackQuery.Set("state", query.Get("state"))
	redirect.RawQuery = callbackQuery.Encode()
	request, _ := http.NewRequest(http.MethodGet, redirect.String(), nil)
	response, err := d.client.Do(request)
	if err != nil {
		return fmt.Errorf("complete loopback callback: %w", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("loopback callback status %d", response.StatusCode)
	}
	return nil
}

func liveQAJSONRequest(client *http.Client, method, target string, input, output any) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, target, body)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Casdoor request status %d", response.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 2<<20))
	if err := decoder.Decode(output); err != nil {
		return errors.New("Casdoor returned invalid JSON")
	}
	return nil
}

package slack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"golang.org/x/time/rate"
)

type ClientOption func(*Client)

type Client struct {
	baseURL      string
	clientID     string
	clientSecret string
	redirectURL  string
	scopes       string
	httpClient   *http.Client
	limiter      *rate.Limiter
	retryMax     int
	retryWait    time.Duration
}

func WithRetry(max int, wait time.Duration) ClientOption {
	return func(c *Client) {
		c.retryMax = max
		c.retryWait = wait
	}
}

func WithRateLimit(rps float64, burst int) ClientOption {
	return func(c *Client) {
		c.limiter = rate.NewLimiter(rate.Limit(rps), burst)
	}
}

func NewSlackClient(baseURL, clientID, clientSecret, redirectURL, scopes string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:      baseURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		scopes:       scopes,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		retryMax:     3,
		retryWait:    time.Second,
		limiter:      rate.NewLimiter(rate.Limit(1), 1),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Client) ResolveChannelID(ctx context.Context, token, channelName string) (string, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return "", fmt.Errorf("rate limit wait: %w", err)
	}

	api := slack.New(token,
		slack.OptionHTTPClient(c.httpClient),
		slack.OptionAppLevelToken(token),
	)

	var channels []slack.Channel
	var err error

	for attempt := 0; attempt <= c.retryMax; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(float64(c.retryWait) * math.Pow(2, float64(attempt)))
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}

		channels, _, err = api.GetConversationsContext(ctx, &slack.GetConversationsParameters{
			Limit:           1000,
			ExcludeArchived: true,
			Types:           []string{"public_channel", "private_channel"},
		})
		if err == nil {
			break
		}
	}
	if err != nil {
		return "", fmt.Errorf("get conversations: %w", err)
	}

	for _, ch := range channels {
		if ch.Name == channelName {
			return ch.ID, nil
		}
	}
	return "", fmt.Errorf("channel %s not found", channelName)
}

func (c *Client) SendMessage(ctx context.Context, token, channelID, message string) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit wait: %w", err)
	}

	api := slack.New(token,
		slack.OptionHTTPClient(c.httpClient),
		slack.OptionAppLevelToken(token),
	)

	var err error
	for attempt := 0; attempt <= c.retryMax; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(float64(c.retryWait) * math.Pow(2, float64(attempt)))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		_, _, err = api.PostMessageContext(ctx, channelID,
			slack.MsgOptionText(message, false),
			slack.MsgOptionDisableLinkUnfurl(),
		)
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("post message: %w", err)
	}
	return nil
}

func (c *Client) ExchangeCode(ctx context.Context, code string) (string, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return "", fmt.Errorf("rate limit wait: %w", err)
	}

	endpoint := "https://slack.com/api/oauth.v2.access"
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", c.clientID)
	data.Set("client_secret", c.clientSecret)
	data.Set("redirect_uri", c.redirectURL)

	var result struct {
		Ok          bool   `json:"ok"`
		Error       string `json:"error"`
		AccessToken string `json:"access_token"`
		AuthedUser  struct {
			AccessToken string `json:"access_token"`
		} `json:"authed_user"`
	}

	var lastErr error
	for attempt := 0; attempt <= c.retryMax; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(float64(c.retryWait) * math.Pow(2, float64(attempt)))
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(data.Encode()))
		if err != nil {
			lastErr = fmt.Errorf("create request: %w", err)
			continue
		}
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http request: %w", err)
			continue
		}

		var respErr error
		func() {
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				respErr = fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
				return
			}

			if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
				respErr = fmt.Errorf("decode response: %w", err)
				return
			}
		}()

		if respErr != nil {
			lastErr = respErr
			continue
		}

		if !result.Ok {
			lastErr = fmt.Errorf("slack error: %s", result.Error)
			continue
		}

		if result.AccessToken != "" {
			return result.AccessToken, nil
		}
		if result.AuthedUser.AccessToken != "" {
			return result.AuthedUser.AccessToken, nil
		}
		lastErr = errors.New("no access token in response")
	}

	return "", fmt.Errorf("exchange code failed after %d attempts: %w", c.retryMax+1, lastErr)
}

func (c *Client) GetOAuthV2URL(state string) (string, error) {
	baseURL := "https://slack.com/oauth/v2/authorize"

	values := url.Values{}
	values.Set("client_id", c.clientID)
	values.Set("redirect_uri", c.redirectURL)
	values.Set("scope", c.scopes)
	values.Set("state", state)

	return baseURL + "?" + values.Encode(), nil
}

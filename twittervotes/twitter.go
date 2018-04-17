package main

import (
	"context"
	"encoding/json"
	"github.com/garyburd/go-oauth/oauth"
	"github.com/joeshaw/envdecode"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var conn net.Conn

var reader io.ReadCloser

var (
	authClient *oauth.Client
	creds      *oauth.Credentials
)

func setupTwitterAuth() {
	var ts struct {
		ConsumerKey    string `env:"SP_TWITTER_KEY,required"`
		ConsumerSecret string `env:"SP_TWITTER_SECRET,required"`
		AccessToken    string `env:"SP_TWITTER_ACCESSTOKEN,required"`
		AccessSecret   string `env:"SP_TWITTER_ACCESSSECRET,required"`
	}
	if err := envdecode.Decode(&ts); err != nil {
		log.Fatalln(err)
	}

	authClient = &oauth.Client{
		Credentials: oauth.Credentials{
			Token:  ts.ConsumerKey,
			Secret: ts.ConsumerSecret,
		},
	}
	creds = &oauth.Credentials{
		Token:  ts.AccessToken,
		Secret: ts.AccessSecret,
	}
}

var (
	authSetupOnce sync.Once
	httpClient    *http.Client
)

func makeRequest(query url.Values) (*http.Request, error) {
	authSetupOnce.Do(func() {
		setupTwitterAuth()
	})

	formEnc := query.Encode()
	req, err := http.NewRequest(
		"POST",
		"https://stream.twitter.com/1.1/statuses/filter.json",
		strings.NewReader(formEnc),
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Content-Length", strconv.Itoa(len(formEnc))) // Itoa stands for "Integer to ASCII"
	// See http://tools.ietf.org/html/rfc5849#section-3.5.1
	authClient.SetAuthorizationHeader(req.Header, creds, "POST", req.URL, query)

	return req, nil
}

type tweet struct {
	Text string
}

// NOTE: We should pass a Context explicitly to each function that needs it.
// The Context should be the first parameter, typically named ctx.
func readFromTwitter(ctx context.Context, votes chan<- string) {
	options, err := loadOptions()
	if err != nil {
		log.Println("選択肢の読み込みに失敗しました:", err)
		return
	}

	query := make(url.Values)
	query.Set("track", strings.Join(options, ","))
	req, err := makeRequest(query)
	if err != nil {
		log.Println("検索のリクエストの作成に失敗しました:", err)
		return
	}

	client := &http.Client{}
	if deadline, ok := ctx.Deadline(); ok {
		client.Timeout = deadline.Sub(time.Now())
	}
	res, err := client.Do(req)
	if err != nil {
		log.Println("検索のリクエストに失敗しました:", err)
		return
	}

	done := make(chan struct{})
	defer func() { <-done }()

	go func() {
		defer close(done)
		log.Println("res:", res.StatusCode)

		if res.StatusCode != 200 {
			body, _ := ioutil.ReadAll(res.Body)
			log.Printf("res body: %s\n", string(body))

			return
		}

		decoder := json.NewDecoder(res.Body)
		for {
			var tweet tweet
			if err := decoder.Decode(&tweet); err != nil {
				break
			}

			log.Println("tweet:", tweet)
			for _, option := range options {
				if strings.Contains(strings.ToLower(tweet.Text), option) {
					log.Println("投票:", option)
					votes <- option
				}
			}
		}
	}()
	defer res.Body.Close()

	select {
	case <-ctx.Done():
	case <-done: // NOTE: receives zero value after the channel is closed
	}
}

func readFromTwitterWithTimeout(ctx context.Context, timeout time.Duration, votes chan<- string) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	readFromTwitter(ctx, votes)
}

func twitterStream(ctx context.Context, votes chan<- string) {
	defer close(votes)

	for {
		log.Println("Twitterに問い合わせます...")
		readFromTwitterWithTimeout(ctx, 1*time.Minute, votes)
		log.Println("（待機中）")

		select {
		case <-ctx.Done():
			log.Println("Twitterへの問い合わせを終了します...")
			return
		case <-time.After(10 * time.Second):
		}
	}
}

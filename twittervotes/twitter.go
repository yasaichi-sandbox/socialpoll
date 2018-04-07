package main

import (
	"context"
	"encoding/json"
	"github.com/garyburd/go-oauth/oauth"
	"github.com/joeshaw/envdecode"
	"io"
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

func dialContext(_ context.Context, netw, addr string) (net.Conn, error) {
	if conn != nil {
		conn.Close()
		conn = nil
	}

	netc, err := net.DialTimeout(netw, addr, 5*time.Second)
	if err != nil {
		return nil, err
	}

	conn = netc
	return netc, nil
}

var reader io.ReadCloser

func closeConn() {
	if conn != nil {
		conn.Close()
	}

	if reader != nil {
		reader.Close()
	}
}

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

func makeRequest(req *http.Request, params url.Values) (*http.Response, error) {
	authSetupOnce.Do(func() {
		setupTwitterAuth()
		httpClient = &http.Client{
			Transport: &http.Transport{
				DialContext: dialContext,
			},
		}
	})

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Content-Length", strconv.Itoa(len(params.Encode()))) // Itoa stands for "Integer to ASCII"

	// See http://tools.ietf.org/html/rfc5849#section-3.5.1
	err := authClient.SetAuthorizationHeader(req.Header, creds, "POST", req.URL, params)
	if err != nil {
		return nil, err
	}

	return httpClient.Do(req)
}

type tweet struct {
	Text string
}

func readFromTwitter(votes chan<- string) {
	options, err := loadOptions()
	if err != nil {
		log.Println("選択肢の読み込みに失敗しました:", err)
		return
	}

	query := make(url.Values)
	query.Set("track", strings.Join(options, ","))
	req, err := http.NewRequest(
		"POST",
		"https://stream.twitter.com/1.1/statuses/filter.json",
		strings.NewReader(query.Encode()),
	)
	if err != nil {
		log.Println("検索のリクエストの作成に失敗しました:", err)
		return
	}

	res, err := makeRequest(req, query)
	if err != nil {
		log.Println("検索のリクエストに失敗しました:", err)
		return
	}

	reader := res.Body
	decoder := json.NewDecoder(reader)
	for {
		var tweet tweet
		if err := decoder.Decode(&tweet); err != nil {
			break
		}

		for _, option := range options {
			if strings.Contains(strings.ToLower(tweet.Text), option) {
				log.Println("投票:", option)
				votes <- option
			}
		}
	}
}

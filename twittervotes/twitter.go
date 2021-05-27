package main

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joeshaw/envdecode"
	"github.com/matryer/go-oauth/oauth"
)

var conn net.Conn

func dial(netw, addr string) (net.Conn, error) {
	if conn != nil {
		conn.Close()
		conn = nil
	}
	netc, err := net.DialTimeout(netw, addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	conn = netc
	return conn, nil
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

func setUpTwitterAuth() {
	var ts struct {
		ConsumerKey    string `env:"SP_TWITTER_KEY,required"`
		ConsumerSecret string `env:"SP_TWITTER_SECRET,required"`
		AccessToken    string `env:"TWITTER_ACCESSTOKEN,required"`
		AccessSecret   string `env:"TWITTER_ACCESSSECRET,required"`
	}
	if err := envdecode.Decode(&ts); err != nil {
		log.Fatalln(err)
	}
	creds = &oauth.Credentials{
		Token:  ts.AccessToken,
		Secret: ts.AccessSecret,
	}
	authClient = &oauth.Client{
		Credentials: oauth.Credentials{
			Token:  ts.ConsumerKey,
			Secret: ts.ConsumerSecret,
		},
	}
}

var (
	authSetupOnce sync.Once
	httpClient    *http.Client
)

// 検索requestを作成する
func makeRequest(req *http.Request, params url.Values) (*http.Response, error) {
	// 初期化(一回しか呼び出されない)
	authSetupOnce.Do(func() {
		setUpTwitterAuth()
		httpClient = &http.Client{
			Transport: &http.Transport{
				Dial: dial,
			},
		}
	})
	// request生成
	formEnc := params.Encode()
	req.Header.Set("Content-type", "application/x-www-form-urlencoded")
	req.Header.Set("Content-Length", strconv.Itoa(len(formEnc)))
	req.Header.Set("Authorization",
		authClient.AuthorizationHeader(creds, "POST", req.URL, params))
	return httpClient.Do(req)
}

// tweetには文字列がありそれだけを扱うことを宣言する
type tweet struct {
	Text string
}

// twitterから読み込み選択肢と被れば投票する
// votesは送信専用
func readFromTwitter(votes chan<- string) {
	// 全ての投票での選択肢を取得している
	options, err := loadOptions()
	if err != nil {
		log.Println("選択肢の読み込みに失敗しました:", err)
		return
	}
	// twitter側のエンドポイントをを指すrawURLを*url.URL型に解析する
	u, err := url.Parse("https://stream.twitter.com/1.1/statuses/filter.json")
	if err != nil {
		log.Println("URLの解析に失敗しました:", err)
		return
	}
	// 選択肢のリスト(options)をカンマ区切りの文字列として指定する
	query := make(url.Values)
	query.Set("track", strings.Join(options, ","))
	req, err := http.NewRequest("POST", u.String(),
		// URLエンコード形式(string)にエンコード
		strings.NewReader(query.Encode()))
	if err != nil {
		log.Println("検索のリクエスト作成に失敗しました:", err)
		return
	}
	resp, err := makeRequest(req, query)
	if err != nil {
		log.Println("検索リクエストに失敗しました:", err)
		return
	}
	// レスポンスの本体をもとにreaderから読み込むdecoderを作成
	reader = resp.Body
	decoder := json.NewDecoder(reader)
	for {
		var tweet tweet
		// tweetの型にdecodeしデーターを読み込む
		if err := decoder.Decode(&tweet); err != nil {
			break
		}
		for _, option := range options {
			// 選択肢のなかにツイートの中で言及されているものがあれば投票する
			if strings.Contains(strings.ToLower(tweet.Text), strings.ToLower(option)) {
				log.Println("投票:", option)
				votes <- option
			}
		}
	}
}

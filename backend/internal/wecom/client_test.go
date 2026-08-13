package wecom

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSendRefreshesRejectedAccessToken(t *testing.T) {
	tokenCalls := 0
	sendCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/cgi-bin/gettoken":
			if request.URL.Query().Get("corpid") != "corp" || request.URL.Query().Get("corpsecret") != "secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			tokenCalls++
			_, _ = w.Write([]byte(`{"errcode":0,"access_token":"token-` + string(rune('0'+tokenCalls)) + `","expires_in":7200}`))
		case "/cgi-bin/appchat/send":
			sendCalls++
			var body struct {
				ChatID string `json:"chatid"`
				Text   struct {
					Content string `json:"content"`
				} `json:"text"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body.ChatID != "chat" || body.Text.Content != "完成" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if request.URL.Query().Get("access_token") == "token-1" {
				_, _ = w.Write([]byte(`{"errcode":42001,"errmsg":"expired"}`))
				return
			}
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "corp", "secret", "chat", time.Second)
	unknown, err := client.Send(context.Background(), "完成")
	if err != nil || unknown {
		t.Fatalf("unknown=%v err=%v", unknown, err)
	}
	if tokenCalls != 2 || sendCalls != 2 {
		t.Fatalf("token calls=%d send calls=%d", tokenCalls, sendCalls)
	}
}

func TestConfigureUpdatesCredentialsAndClearsCachedToken(t *testing.T) {
	tokenCalls := 0
	seen := make([][2]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/cgi-bin/gettoken" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		tokenCalls++
		seen = append(seen, [2]string{request.URL.Query().Get("corpid"), request.URL.Query().Get("corpsecret")})
		_, _ = w.Write([]byte(`{"errcode":0,"access_token":"token-` + string(rune('0'+tokenCalls)) + `","expires_in":7200}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "old-corp", "old-secret", "old-chat", time.Second)
	if _, err := client.accessToken(context.Background()); err != nil {
		t.Fatalf("initial token: %v", err)
	}
	if _, err := client.accessToken(context.Background()); err != nil {
		t.Fatalf("cached token: %v", err)
	}
	if tokenCalls != 1 {
		t.Fatalf("cached token was not reused: token calls=%d", tokenCalls)
	}

	client.Configure(server.URL, "new-corp", "new-secret", "new-chat")
	if !client.Configured() {
		t.Fatal("client should remain configured after credential update")
	}
	token, err := client.accessToken(context.Background())
	if err != nil {
		t.Fatalf("token after configure: %v", err)
	}
	if token != "token-2" {
		t.Fatalf("token = %q, want token-2", token)
	}
	if tokenCalls != 2 {
		t.Fatalf("configure did not clear cached token: token calls=%d", tokenCalls)
	}
	if seen[0] != [2]string{"old-corp", "old-secret"} || seen[1] != [2]string{"new-corp", "new-secret"} {
		t.Fatalf("token credentials = %#v", seen)
	}
}

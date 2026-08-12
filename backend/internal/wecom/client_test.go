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

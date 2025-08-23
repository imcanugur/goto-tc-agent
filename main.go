package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type ReqMsg struct {
	Type    string              `json:"type"`
	ID      string              `json:"id"`
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Headers map[string][]string `json:"headers"`
	BodyB64 string              `json:"body_b64"`
}

type RespMsg struct {
	Type    string              `json:"type"`
	ID      string              `json:"id"`
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers"`
	BodyB64 string              `json:"body_b64"`
}

const serverDomain = "goto.tc"

func startAgent(target string, tunnel string) {
	serverURL := "wss://" + serverDomain + "/ws/agent?tunnel=" + url.QueryEscape(tunnel)

	conn, _, err := websocket.DefaultDialer.Dial(serverURL, nil)
	if err != nil {
		log.Fatal("Agent bağlantı hatası:", err)
	}
	defer conn.Close()

	tunnelURL := fmt.Sprintf("https://%s.%s", tunnel, serverDomain)
	log.Printf("🚀 Tünel aktif: %s → %s\n", tunnelURL, target)

	httpClient := &http.Client{Timeout: 30 * time.Second}

	for {
		var req ReqMsg
		if err := conn.ReadJSON(&req); err != nil {
			log.Println("Sunucudan okuma hatası:", err)
			return
		}

		localURL := target + req.Path
		httpReq, _ := http.NewRequest(req.Method, localURL, nil)
		for k, vals := range req.Headers {
			for _, v := range vals {
				httpReq.Header.Add(k, v)
			}
		}
		if req.BodyB64 != "" {
			body, _ := base64.StdEncoding.DecodeString(req.BodyB64)
			httpReq.Body = io.NopCloser(strings.NewReader(string(body)))
			httpReq.ContentLength = int64(len(body))
		}

		resp, err := httpClient.Do(httpReq)
		if err != nil {
			conn.WriteJSON(RespMsg{
				Type: "response", ID: req.ID, Status: 502,
				Headers: map[string][]string{"X-Agent-Error": {err.Error()}},
			})
			continue
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		conn.WriteJSON(RespMsg{
			Type:    "response",
			ID:      req.ID,
			Status:  resp.StatusCode,
			Headers: resp.Header,
			BodyB64: base64.StdEncoding.EncodeToString(body),
		})
	}
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Kullanım: agent <targetURL>")
	}

	target := os.Args[1]
	tunnel := fmt.Sprintf("demo-%d", time.Now().Unix()%10000)

	startAgent(target, tunnel)
}

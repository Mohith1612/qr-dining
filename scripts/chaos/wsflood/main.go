// wsflood is a chaos driver for the qr-dining WebSocket layer.
//
//	flood mode  — opens one connection and sends N frames as fast as possible to
//	              exercise per-connection inbound rate limiting + malformed-frame
//	              strike budget. Reports when/if the server force-closes us.
//	storm mode  — attempts C ticket+connect cycles rapidly to exercise the
//	              per-session ws-ticket reconnect-burst limit. Reports how many
//	              connected vs. were throttled (429) vs. rejected.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	base := flag.String("base", "http://localhost:8080", "app base URL")
	session := flag.String("session", "", "session id")
	token := flag.String("token", "", "guest access token")
	mode := flag.String("mode", "flood", "flood|storm")
	n := flag.Int("n", 500, "frames to send (flood)")
	c := flag.Int("c", 30, "connection attempts (storm)")
	malformed := flag.Bool("malformed", true, "send malformed frames (flood)")
	flag.Parse()

	switch *mode {
	case "flood":
		runFlood(*base, *session, *token, *n, *malformed)
	case "storm":
		runStorm(*base, *session, *token, *c)
	default:
		fmt.Println("unknown mode")
	}
}

func issueTicket(base, session, token string) (string, int, error) {
	req, _ := http.NewRequest("POST", base+"/sessions/"+session+"/ws-ticket", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", resp.StatusCode, nil
	}
	var out struct {
		Ticket string `json:"ticket"`
	}
	_ = json.Unmarshal(body, &out)
	return out.Ticket, resp.StatusCode, nil
}

func wsURL(base, ticket string) string {
	u, _ := url.Parse(base)
	scheme := "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}
	return fmt.Sprintf("%s://%s/ws?ticket=%s", scheme, u.Host, ticket)
}

func runFlood(base, session, token string, n int, malformed bool) {
	ticket, code, err := issueTicket(base, session, token)
	if err != nil || ticket == "" {
		fmt.Printf("flood: could not get ticket (http=%d err=%v)\n", code, err)
		return
	}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL(base, ticket), nil)
	if err != nil {
		fmt.Printf("flood: dial failed: %v\n", err)
		return
	}
	defer conn.Close()

	// Reader: detect server-initiated close.
	closedAt := make(chan int, 1)
	var sent int64
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				closedAt <- int(atomic.LoadInt64(&sent))
				return
			}
		}
	}()

	payload := []byte(`{"event":"spam","junk":`)        // malformed (unterminated)
	good := []byte(`{"event":"ping"}`)                  // well-formed but pointless
	frame := good
	if malformed {
		frame = payload
	}

	start := time.Now()
	for i := 0; i < n; i++ {
		if err := conn.WriteMessage(websocket.TextMessage, frame); err != nil {
			fmt.Printf("flood: write failed after %d frames (server closed): %v\n", i, err)
			fmt.Printf("RESULT flood: server_closed=true frames_sent=%d elapsed=%s\n", i, time.Since(start))
			return
		}
		atomic.AddInt64(&sent, 1)
	}

	select {
	case at := <-closedAt:
		fmt.Printf("RESULT flood: server_closed=true after≈%d frames elapsed=%s\n", at, time.Since(start))
	case <-time.After(2 * time.Second):
		fmt.Printf("RESULT flood: server_closed=false frames_sent=%d (no force-close within budget) elapsed=%s\n", n, time.Since(start))
	}
}

func runStorm(base, session, token string, c int) {
	var connected, throttled, rejected int64
	var wg sync.WaitGroup
	for i := 0; i < c; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticket, code, err := issueTicket(base, session, token)
			if err != nil {
				atomic.AddInt64(&rejected, 1)
				return
			}
			if code == http.StatusTooManyRequests {
				atomic.AddInt64(&throttled, 1)
				return
			}
			if ticket == "" {
				atomic.AddInt64(&rejected, 1)
				return
			}
			conn, _, err := websocket.DefaultDialer.Dial(wsURL(base, ticket), nil)
			if err != nil {
				atomic.AddInt64(&rejected, 1)
				return
			}
			atomic.AddInt64(&connected, 1)
			time.Sleep(200 * time.Millisecond)
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			conn.Close()
		}()
	}
	wg.Wait()
	fmt.Printf("RESULT storm: attempts=%d connected=%d throttled_429=%d rejected=%d\n",
		c, connected, throttled, rejected)
}

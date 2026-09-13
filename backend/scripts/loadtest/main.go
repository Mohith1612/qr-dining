// Command loadtest drives synthetic pilot-shaped traffic against a running
// qr-dining backend and reports client-side latency percentiles and error rates.
//
// It is a load *driver* only — it never touches the soak; point it at an isolated
// stack. Flow per session: create session -> GET menu -> add cart items ->
// place order -> ws-ticket + WS connect + read -> snapshot (reconnect) ->
// initiate payment (cash).
//
// Env:
//
//	BASE_URL        http base (default http://127.0.0.1:18080)
//	WS_URL          ws base   (default ws://127.0.0.1:18080)
//	BRANCH_ID       branch id for menu/order (required)
//	TABLE_IDS       csv of table ids to spread sessions across (required)
//	MENU_ITEM_IDS   csv of menu item ids to order (required)
//	CONCURRENCY     parallel workers (default 25)
//	DURATION        seconds to run (default 20)
//	WS_HOLD         seconds each ws connection is held (default 3)
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

var (
	baseURL  = env("BASE_URL", "http://127.0.0.1:18080")
	wsURL    = env("WS_URL", "ws://127.0.0.1:18080")
	branchID = env("BRANCH_ID", "")
	tableIDs = splitInts(env("TABLE_IDS", ""))
	itemIDs  = splitInts(env("MENU_ITEM_IDS", ""))
	// Optional table-id range (avoids exhausting tables via the one-active-per-table
	// invariant during sustained load). If set, overrides TABLE_IDS.
	tableMin, _ = strconv.ParseInt(env("TABLE_ID_MIN", "0"), 10, 64)
	tableMax, _ = strconv.ParseInt(env("TABLE_ID_MAX", "0"), 10, 64)
)

func pickTable(rng *rand.Rand) int64 {
	if tableMax > tableMin && tableMin > 0 {
		return tableMin + rng.Int63n(tableMax-tableMin+1)
	}
	return tableIDs[rng.Intn(len(tableIDs))]
}

type stat struct {
	mu  sync.Mutex
	lat []float64 // ms
	ok  int64
	err int64
}

func (s *stat) add(d time.Duration, err error) {
	if err != nil {
		atomic.AddInt64(&s.err, 1)
		return
	}
	atomic.AddInt64(&s.ok, 1)
	s.mu.Lock()
	s.lat = append(s.lat, float64(d.Microseconds())/1000.0)
	s.mu.Unlock()
}

func (s *stat) report(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sort.Float64s(s.lat)
	fmt.Printf("%-16s ok=%-6d err=%-5d p50=%-7.1f p95=%-7.1f p99=%-7.1f max=%-7.1f ms\n",
		name, s.ok, s.err, pct(s.lat, 50), pct(s.lat, 95), pct(s.lat, 99), pct(s.lat, 100))
}

func pct(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	i := (p * (len(sorted) - 1)) / 100
	return sorted[i]
}

var (
	stCreate   = &stat{}
	stMenu     = &stat{}
	stCart     = &stat{}
	stOrder    = &stat{}
	stTicket   = &stat{}
	stWS       = &stat{}
	stSnap     = &stat{}
	stPay      = &stat{}
	httpClient = &http.Client{
		Timeout:   15 * time.Second,
		Transport: &http.Transport{MaxIdleConns: 500, MaxIdleConnsPerHost: 500, MaxConnsPerHost: 500},
	}
)

func main() {
	if branchID == "" || len(itemIDs) == 0 || (len(tableIDs) == 0 && tableMax <= tableMin) {
		fmt.Fprintln(os.Stderr, "BRANCH_ID, MENU_ITEM_IDS and (TABLE_IDS or TABLE_ID_MIN/MAX) are required")
		os.Exit(2)
	}
	concurrency, _ := strconv.Atoi(env("CONCURRENCY", "25"))
	duration, _ := strconv.Atoi(env("DURATION", "20"))
	wsHold, _ := strconv.Atoi(env("WS_HOLD", "3"))

	fmt.Printf("loadtest: base=%s ws=%s branch=%s tables=%d items=%d concurrency=%d duration=%ds\n",
		baseURL, wsURL, branchID, len(tableIDs), len(itemIDs), concurrency, duration)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(duration)*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var flows int64
	start := time.Now()
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(seed) + time.Now().UnixNano()))
			for ctx.Err() == nil {
				runFlow(ctx, rng, wsHold)
				atomic.AddInt64(&flows, 1)
			}
		}(w)
	}
	wg.Wait()
	elapsed := time.Since(start).Seconds()

	fmt.Printf("\n=== results (%.1fs, %d flows, %.1f flows/s) ===\n", elapsed, flows, float64(flows)/elapsed)
	stCreate.report("session.create")
	stMenu.report("menu.get")
	stCart.report("cart.add")
	stOrder.report("order.place")
	stTicket.report("ws.ticket")
	stWS.report("ws.connect")
	stSnap.report("snapshot")
	stPay.report("payment.init")

	totalErr := stCreate.err + stMenu.err + stCart.err + stOrder.err + stTicket.err + stWS.err + stSnap.err + stPay.err
	totalOK := stCreate.ok + stMenu.ok + stCart.ok + stOrder.ok + stTicket.ok + stWS.ok + stSnap.ok + stPay.ok
	rate := 0.0
	if totalOK+totalErr > 0 {
		rate = 100 * float64(totalErr) / float64(totalOK+totalErr)
	}
	fmt.Printf("\ntotal ops ok=%d err=%d error_rate=%.2f%%\n", totalOK, totalErr, rate)
}

func runFlow(ctx context.Context, rng *rand.Rand, wsHold int) {
	tableID := pickTable(rng)
	// Each flow simulates a distinct guest device: a unique client IP so per-IP
	// rate limits behave as in production (one IP per guest), not as one hammering IP.
	xff := fmt.Sprintf("10.%d.%d.%d", rng.Intn(254)+1, rng.Intn(254)+1, rng.Intn(254)+1)
	call := func(method, path, token string, body, out any) error {
		return httpDo(ctx, method, path, token, body, out, xff)
	}

	// 1. create session
	body := map[string]any{"table_id": tableID, "display_name": fmt.Sprintf("Load%d", rng.Intn(100000))}
	var cs struct {
		Session struct {
			ID       string `json:"id"`
			BranchID int64  `json:"branch_id"`
		} `json:"session"`
		Participant struct {
			ID int64 `json:"id"`
		} `json:"participant"`
		Token string `json:"guest_access_token"`
	}
	t := time.Now()
	err := call("POST", "/sessions", "", body, &cs)
	stCreate.add(time.Since(t), err)
	if err != nil || cs.Session.ID == "" {
		return
	}
	sid, tok := cs.Session.ID, cs.Token

	// 2. menu
	t = time.Now()
	err = call("GET", "/branches/"+branchID+"/menu", tok, nil, nil)
	stMenu.add(time.Since(t), err)

	// 3. add 1-2 cart items
	n := 1 + rng.Intn(2)
	for i := 0; i < n; i++ {
		item := itemIDs[rng.Intn(len(itemIDs))]
		t = time.Now()
		err = call("POST", "/sessions/"+sid+"/cart/items", tok,
			map[string]any{"menu_item_id": item, "quantity": 1}, nil)
		stCart.add(time.Since(t), err)
	}

	// 4. place order (branch_id + placed_by are server-derived from the guest token)
	item := itemIDs[rng.Intn(len(itemIDs))]
	idem := fmt.Sprintf("load-%s-%d", sid, time.Now().UnixNano())
	order := map[string]any{
		"idempotency_key": idem,
		"items":           []map[string]any{{"menu_item_id": item, "quantity": 1}},
	}
	t = time.Now()
	err = call("POST", "/sessions/"+sid+"/orders", tok, order, nil)
	stOrder.add(time.Since(t), err)

	// 5. ws-ticket + connect + read
	var tk struct {
		Ticket string `json:"ticket"`
	}
	t = time.Now()
	err = call("POST", "/sessions/"+sid+"/ws-ticket", tok, map[string]any{}, &tk)
	stTicket.add(time.Since(t), err)
	if err == nil && tk.Ticket != "" {
		t = time.Now()
		werr := wsConnect(tk.Ticket, wsHold)
		stWS.add(time.Since(t), werr)
	}

	// 6. snapshot (reconnect reconcile)
	t = time.Now()
	err = call("GET", "/sessions/"+sid+"/snapshot?last_sequence=0", tok, nil, nil)
	stSnap.add(time.Since(t), err)

	// 7. fetch bill, then initiate payment for the exact total (cash -> requires staff confirmation)
	var bill struct {
		Total float64 `json:"total"`
	}
	if err = call("GET", "/sessions/"+sid+"/bill", tok, nil, &bill); err != nil || bill.Total <= 0 {
		return
	}
	pidem := fmt.Sprintf("pay-%s-%d", sid, time.Now().UnixNano())
	pay := map[string]any{"amount": bill.Total, "method": "cash", "idempotency_key": pidem}
	t = time.Now()
	err = call("POST", "/sessions/"+sid+"/payments", tok, pay, nil)
	stPay.add(time.Since(t), err)
}

func wsConnect(ticket string, holdSec int) error {
	d := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	c, _, err := d.Dial(wsURL+"/ws?ticket="+ticket, nil)
	if err != nil {
		return err
	}
	defer c.Close()
	deadline := time.Now().Add(time.Duration(holdSec) * time.Second)
	_ = c.SetReadDeadline(deadline)
	for time.Now().Before(deadline) {
		if _, _, err := c.ReadMessage(); err != nil {
			break // timeout/close expected
		}
	}
	return nil
}

func httpDo(ctx context.Context, method, path, token string, body, out any, xff string) error {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	req.Header.Set("X-Tenant-Slug", "loadtest")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("status %d: %s", resp.StatusCode, truncate(string(data), 120))
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func splitInts(s string) []int64 {
	var out []int64
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if v, err := strconv.ParseInt(p, 10, 64); err == nil {
			out = append(out, v)
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

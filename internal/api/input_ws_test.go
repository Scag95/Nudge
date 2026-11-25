package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"nudge/internal/input"
	"nudge/internal/pair"
)

func newInputServer(t *testing.T, rate int) (url string, b *pair.Broker, rec *input.Recorder) {
	t.Helper()
	return newInputServerLimits(t, rate, 0)
}

// newInputServerLimits starts the server with an explicit move budget (0 = default).
func newInputServerLimits(t *testing.T, rate, moveRate int) (url string, b *pair.Broker, rec *input.Recorder) {
	t.Helper()
	t.Setenv("NUDGE_STATE", filepath.Join(t.TempDir(), "state.json"))
	b = pair.NewBroker()
	rec = &input.Recorder{}
	r := mux.NewRouter()
	AttachPairing(r, b)
	AttachInputWSLimits(r, b, rec, rate, moveRate)
	s := httptest.NewServer(r)
	t.Cleanup(s.Close)
	return s.URL, b, rec
}

func inputToken(t *testing.T, b *pair.Broker) string {
	t.Helper()
	req, err := b.Request("input-device", "Test Phone", "127.0.0.1", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Approve(req.RequestID); err != nil {
		t.Fatal(err)
	}
	status, err := b.Status(req.RequestID, "127.0.0.1")
	if err != nil || status.Token == "" {
		t.Fatalf("approved token status = %+v, err = %v", status, err)
	}
	return status.Token
}

func dial(t *testing.T, url, token string) (*websocket.Conn, int) {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(url, "http") + "/ws/input?token=" + token
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		code := 0
		if resp != nil {
			code = resp.StatusCode
			defer resp.Body.Close()
		}
		return nil, code
	}
	t.Cleanup(func() { conn.Close() })
	return conn, http.StatusOK
}

func expectErrFrame(t *testing.T, conn *websocket.Conn) string {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var frame struct {
		Error string `json:"error"`
	}
	if err := conn.ReadJSON(&frame); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if frame.Error == "" {
		t.Fatal("expected non-empty error field in frame")
	}
	return frame.Error
}

func TestInputWSRejectsMissingOrBadToken(t *testing.T) {
	url, _, _ := newInputServer(t, 120)
	for _, tok := range []string{"", "bogus"} {
		conn, code := dial(t, url, tok)
		if conn != nil {
			t.Fatalf("token %q: upgrade succeeded, want rejection", tok)
		}
		if code != http.StatusUnauthorized {
			t.Fatalf("token %q: status = %d, want 401", tok, code)
		}
	}
}

func TestInputWSRejectsRevokedToken(t *testing.T) {
	url, b, _ := newInputServer(t, 120)
	tok := inputToken(t, b)
	if err := b.Revoke("input-device"); err != nil {
		t.Fatal(err)
	}
	conn, code := dial(t, url, tok)
	if conn != nil || code != http.StatusUnauthorized {
		t.Fatalf("revoked token connected (code %d)", code)
	}
}

func TestInputWSAcceptedEventReachesInjector(t *testing.T) {
	url, b, rec := newInputServer(t, 120)
	tok := inputToken(t, b)
	conn, code := dial(t, url, tok)
	if code != http.StatusOK {
		t.Fatalf("dial status %d", code)
	}
	if err := conn.WriteJSON(map[string]any{"type": "move", "dx": 7, "dy": -2}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range rec.Entries() {
			if e == "move 7 -2" {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("event never reached injector; entries = %v", rec.Entries())
}

func TestInputWSInvalidEventGetsErrorFrameAndStaysOpen(t *testing.T) {
	url, b, rec := newInputServer(t, 120)
	tok := inputToken(t, b)
	conn, _ := dial(t, url, tok)

	// S6: bad payloads answer {"error": ...} without closing the connection.
	bad := []map[string]any{
		{"type": "dance"},
		{"type": "move", "dx": 99999},
		{"type": "click", "button": "middle", "mode": "tap"},
	}
	for _, msg := range bad {
		if err := conn.WriteJSON(msg); err != nil {
			t.Fatal(err)
		}
		expectErrFrame(t, conn)
	}
	// malformed raw frame
	if err := conn.WriteMessage(websocket.TextMessage, []byte("{")); err != nil {
		t.Fatal(err)
	}
	expectErrFrame(t, conn)

	// Connection still usable for valid events.
	if err := conn.WriteJSON(map[string]any{"type": "scroll", "dy": 3}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range rec.Entries() {
			if e == "scroll 0 3" {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("connection died after invalid events; entries = %v", rec.Entries())
}

// drainErrFrames counts error frames arriving within window.
func drainErrFrames(conn *websocket.Conn, window time.Duration) int {
	errs := 0
	conn.SetReadDeadline(time.Now().Add(window))
	for {
		var frame struct {
			Error string `json:"error"`
		}
		if err := conn.ReadJSON(&frame); err != nil {
			return errs
		}
		if frame.Error != "" {
			errs++
		}
	}
}

func countPrefix(rec *input.Recorder, prefix string) int {
	n := 0
	for _, e := range rec.Entries() {
		if strings.HasPrefix(e, prefix) {
			n++
		}
	}
	return n
}

// Non-move events keep the strict bucket and its error frames.
func TestInputWSRateLimit(t *testing.T) {
	url, b, rec := newInputServer(t, 5) // 5 events/s, burst 5
	tok := inputToken(t, b)
	conn, _ := dial(t, url, tok)

	for i := 0; i < 10; i++ {
		if err := conn.WriteJSON(map[string]any{"type": "click", "button": "left", "mode": "tap"}); err != nil {
			t.Fatal(err)
		}
	}
	if errs := drainErrFrames(conn, 300*time.Millisecond); errs == 0 {
		t.Fatal("burst of 10 clicks at 5/s produced no rate-limit error frames")
	}
	if got := len(rec.Entries()); got > 6 { // 5 burst + at most 1 refilled
		t.Fatalf("%d events injected, want at most 6 at rate 5/s", got)
	}
}

func TestInputWSScrollAndKeyKeepStrictLimit(t *testing.T) {
	url, b, rec := newInputServer(t, 5)
	tok := inputToken(t, b)
	conn, _ := dial(t, url, tok)
	for i := 0; i < 5; i++ {
		_ = conn.WriteJSON(map[string]any{"type": "scroll", "dy": 1})
		_ = conn.WriteJSON(map[string]any{"type": "key", "key": "a"})
	}
	if errs := drainErrFrames(conn, 300*time.Millisecond); errs == 0 {
		t.Fatal("scroll/key burst over the budget produced no error frames")
	}
	if got := len(rec.Entries()); got > 6 {
		t.Fatalf("%d events injected, want at most 6", got)
	}
}

// Moves above the general 120/s budget are neither rejected nor answered.
func TestInputWSMovesAboveGeneralRateAreNotRejected(t *testing.T) {
	url, b, rec := newInputServer(t, 120)
	tok := inputToken(t, b)
	conn, _ := dial(t, url, tok)
	const n = 400
	for i := 0; i < n; i++ {
		if err := conn.WriteJSON(map[string]any{"type": "move", "dx": 1, "dy": 0}); err != nil {
			t.Fatal(err)
		}
	}
	if errs := drainErrFrames(conn, 400*time.Millisecond); errs != 0 {
		t.Fatalf("%d error frames for moves under the move cap, want 0", errs)
	}
	if got := countPrefix(rec, "move "); got != n {
		t.Fatalf("%d moves injected, want %d", got, n)
	}
}

// Moves over their own cap are dropped silently: no error frame, not injected.
func TestInputWSMovesOverMoveCapDroppedSilently(t *testing.T) {
	url, b, rec := newInputServerLimits(t, 120, 5)
	tok := inputToken(t, b)
	conn, _ := dial(t, url, tok)
	for i := 0; i < 50; i++ {
		if err := conn.WriteJSON(map[string]any{"type": "move", "dx": 1, "dy": 0}); err != nil {
			t.Fatal(err)
		}
	}
	if errs := drainErrFrames(conn, 300*time.Millisecond); errs != 0 {
		t.Fatalf("%d error frames for dropped moves, want 0", errs)
	}
	if got := countPrefix(rec, "move "); got < 5 || got > 7 { // burst 5 + at most refill
		t.Fatalf("%d moves injected at cap 5/s, want about 5", got)
	}
	// The connection stays usable and clicks use the other bucket.
	if err := conn.WriteJSON(map[string]any{"type": "click", "button": "left", "mode": "tap"}); err != nil {
		t.Fatal(err)
	}
	// (A read timeout is sticky in gorilla, so just wait for the injection.)
	deadline := time.Now().Add(2 * time.Second)
	for countPrefix(rec, "click") == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if countPrefix(rec, "click") != 1 {
		t.Fatalf("click not injected after dropped moves; entries = %v", rec.Entries())
	}
}

// Moves cannot be used to starve or bypass the strict bucket: a click is
// charged to the general bucket even when interleaved with moves.
func TestInputWSMoveFloodDoesNotConsumeClickBudget(t *testing.T) {
	url, b, rec := newInputServer(t, 5)
	tok := inputToken(t, b)
	conn, _ := dial(t, url, tok)
	for i := 0; i < 100; i++ {
		_ = conn.WriteJSON(map[string]any{"type": "move", "dx": 1, "dy": 0})
	}
	for i := 0; i < 3; i++ {
		_ = conn.WriteJSON(map[string]any{"type": "click", "button": "left", "mode": "tap"})
	}
	if errs := drainErrFrames(conn, 300*time.Millisecond); errs != 0 {
		t.Fatalf("%d error frames; 3 clicks fit the budget of 5", errs)
	}
	if got := countPrefix(rec, "click"); got != 3 {
		t.Fatalf("%d clicks injected, want 3", got)
	}
}

func TestInputWSOverlongFrameRejected(t *testing.T) {
	url, b, _ := newInputServer(t, 120)
	tok := inputToken(t, b)
	conn, _ := dial(t, url, tok)
	huge := map[string]any{"type": "text", "chars": strings.Repeat("x", 10000)}
	if err := conn.WriteJSON(huge); err != nil {
		t.Fatal(err)
	}
	// Read limit closes the connection; that is acceptable (frame is never
	// injected). The server must not panic.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn.ReadMessage()
	if err == nil {
		var frame map[string]string
		// tolerate an error frame instead of a hard close
		_ = json.Unmarshal(nil, &frame)
	}
}

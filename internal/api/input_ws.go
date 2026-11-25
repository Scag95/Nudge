package api

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"nudge/internal/input"
)

// DefaultInputRate is the per-connection event budget when none is given.
const DefaultInputRate = 120

// DefaultMoveRate is the per-connection budget for `move` events alone: a
// 120 Hz touch panel (or several coalesced sources) must never trip the strict
// bucket above. Moves over it are dropped silently.
const DefaultMoveRate = 1000

// moveDropLogEvery throttles the log line for silently dropped moves.
const moveDropLogEvery = 1000

// maxInputFrameBytes bounds one protocol message; larger frames are dropped
// by the connection and never parsed or injected.
const maxInputFrameBytes = 4096

var inputUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// The LAN is untrusted enough that the session token, not the origin
	// header, is the authority. Browsers on this LAN could forge Origin
	// anyway; only the pairing client (Android) holds tokens.
	CheckOrigin: func(*http.Request) bool { return true },
}

// AttachInputWS registers the remote-control event stream (S3, S4, S6).
// GET /ws/input?token=<session token> upgrades to a WebSocket only when the
// token was issued by pairing; otherwise the request is rejected with 401
// before the upgrade. Validated events are injected at most
// eventsPerSecond; violations answer {"error":"..."} frames and are never
// injected. eventsPerSecond <= 0 selects DefaultInputRate.
func AttachInputWS(r *mux.Router, v TokenValidator, inj input.Injector, eventsPerSecond int) {
	AttachInputWSLimits(r, v, inj, eventsPerSecond, DefaultMoveRate)
}

// AttachInputWSLimits is AttachInputWS with an explicit budget for `move`
// events. Moves have their own, higher bucket and are dropped silently when
// it is empty; every other event kind (and every invalid frame) uses the
// eventsPerSecond bucket and answers an error frame when over it.
func AttachInputWSLimits(r *mux.Router, v TokenValidator, inj input.Injector, eventsPerSecond, movesPerSecond int) {
	if eventsPerSecond <= 0 {
		eventsPerSecond = DefaultInputRate
	}
	if movesPerSecond <= 0 {
		movesPerSecond = DefaultMoveRate
	}
	h := &inputHandlers{validator: v, injector: inj, rate: eventsPerSecond, moveRate: movesPerSecond}
	r.HandleFunc("/ws/input", h.serve)
}

type inputHandlers struct {
	validator TokenValidator
	injector  input.Injector
	rate      int
	moveRate  int
}

func (h *inputHandlers) serve(w http.ResponseWriter, r *http.Request) {
	if !h.validator.Validate(r.URL.Query().Get("token")) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	conn, err := inputUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade already wrote the error
	}
	defer conn.Close()
	conn.SetReadLimit(maxInputFrameBytes)

	// Two budgets per connection: `move` has its own generous one (a touch
	// panel at 120 Hz or more produces them continuously and a dropped move
	// only costs a few pixels), everything else keeps the strict one.
	// The kind is taken from the parsed event, never from the raw bytes, so a
	// frame cannot pick the cheaper bucket by lying about its type. Frames are
	// already bounded by the read limit; invalid ones are charged to the
	// strict bucket and answered, exactly as before.
	strict := newBucket(h.rate, time.Now())
	moves := newBucket(h.moveRate, time.Now())
	droppedMoves := 0
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return // client gone or frame over the read limit
		}
		cmd, perr := input.Parse(data)
		if _, isMove := cmd.(input.Move); perr == nil && isMove {
			if !moves.allow(time.Now()) {
				droppedMoves++
				if droppedMoves%moveDropLogEvery == 1 {
					log.Printf("input: dropped %d move events over %d/s so far", droppedMoves, h.moveRate)
				}
				continue
			}
		} else if !strict.allow(time.Now()) {
			h.writeError(conn, "rate limit exceeded")
			continue
		}
		if perr != nil {
			h.writeError(conn, perr.Error())
			continue
		}
		if err := cmd.Run(h.injector); err != nil {
			h.writeError(conn, "inject failed: "+err.Error())
		}
	}
}

func (h *inputHandlers) writeError(conn *websocket.Conn, msg string) {
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_ = conn.WriteJSON(map[string]string{"error": msg})
}

// bucket is a token bucket: burst capacity equals the per-second rate.
// Single-goroutine use (one per connection); no locking needed.
type bucket struct {
	rate   float64
	max    float64
	credit float64
	last   time.Time
}

func newBucket(rate int, now time.Time) *bucket {
	return &bucket{
		rate:   float64(rate),
		max:    float64(rate),
		credit: float64(rate),
		last:   now,
	}
}

func (b *bucket) allow(now time.Time) bool {
	b.credit += now.Sub(b.last).Seconds() * b.rate
	if b.credit > b.max {
		b.credit = b.max
	}
	b.last = now
	if b.credit < 1 {
		return false
	}
	b.credit--
	return true
}

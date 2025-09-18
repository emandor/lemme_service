package ws

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/emandor/lemme_service/internal/providers"
	"github.com/emandor/lemme_service/internal/telemetry"
	"github.com/gofiber/contrib/websocket"
)

var (
	mu    sync.RWMutex
	rooms = map[string]map[*websocket.Conn]struct{}{}
)

type Action string

const (
	ActionJoin  Action = "join"
	ActionLeave Action = "leave"
)

type Room string

const (
	RoomQuiz     Room = "quiz.room"
	RoomQuizUser Room = "quiz.room.user"
)

type Event string

const (
	EventQuizCreated     Event = "quiz.event.created"
	EventQuizOCRDone     Event = "quiz.event.ocr_done"
	EventQuizAnswerAdded Event = "quiz.event.answered"
	EventQuizCompleted   Event = "quiz.event.completed"
	EventQuizError       Event = "quiz.event.error"
)

type PayloadEvent struct {
	Event  Event                `json:"event"`
	Source providers.SourceName `json:"source,omitempty"`
	Data   any                  `json:"data,omitempty"`
}

type ClientMessage struct {
	Action Action `json:"action"`
	Room   string `json:"room"`
}

func HandleWS(c *websocket.Conn) {
	tlog := telemetry.L().With().Str("module", "ws").Logger()
	tlog.Info().Msg("ws_connected")
	defer func() {
		// cleanup on disconnect
		mu.Lock()
		for room := range rooms {
			delete(rooms[room], c)
		}
		mu.Unlock()
		_ = c.Close()
	}()

	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			break
		}

		var cm ClientMessage
		if err := json.Unmarshal(msg, &cm); err != nil {
			continue
		}

		switch cm.Action {
		case ActionJoin:
			joinRoom(c, cm.Room)
		case ActionLeave:
			leaveRoom(c, cm.Room)
		}
	}
}

func joinRoom(c *websocket.Conn, room string) {
	if room == "" {
		return
	}
	mu.Lock()
	if rooms[room] == nil {
		rooms[room] = map[*websocket.Conn]struct{}{}
	}
	rooms[room][c] = struct{}{}
	mu.Unlock()
	fmt.Printf("conn joined room: %s\n", room)
}

func HasSubscribers(quizID int64) bool {
	room := string(RoomQuiz) + "." + strconv.FormatInt(quizID, 10)
	mu.RLock()
	defer mu.RUnlock()
	return len(rooms[room]) > 0
}

func leaveRoom(c *websocket.Conn, room string) {
	if room == "" {
		return
	}
	mu.Lock()
	delete(rooms[room], c)
	mu.Unlock()
	fmt.Printf("conn left room: %s\n", room)
}

func BroadcastNewQuiz(userID, quizID int64, image string) {
	userRoom := string(RoomQuizUser) + "." + strconv.FormatInt(userID, 10)
	pl := PayloadEvent{
		Event: EventQuizCreated,
		Data: map[string]any{
			"quiz_id":    quizID,
			"image_path": image,
		},
	}

	mu.RLock()
	conns := rooms[userRoom]
	mu.RUnlock()

	for c := range conns {
		_ = c.WriteJSON(pl)
	}
}

type QuizUpdatePayload struct {
	QuizID  int64  `json:"quiz_id"`
	OCRText string `json:"ocr_text,omitempty"`
	Error   string `json:"error,omitempty"`
}

func BroadcastQuizUpdate(quizID int64, source providers.SourceName, ans *providers.Answer, err error) {
	room := string(RoomQuiz) + "." + strconv.FormatInt(quizID, 10)
	if ans == nil {
		ans = &providers.Answer{}
	}

	ans.QuizID = quizID

	pl := PayloadEvent{
		Event:  EventQuizAnswerAdded,
		Source: source,
		Data:   ans,
	}

	if err != nil {
		pl.Event = EventQuizError
		pl.Data = err.Error()
	}

	mu.RLock()
	conns := rooms[room]
	mu.RUnlock()

	for c := range conns {
		_ = c.WriteJSON(pl)
	}
}

func BroadcastQuizCompleted(quizID int64) {
	room := string(RoomQuiz) + "." + strconv.FormatInt(quizID, 10)

	pl := PayloadEvent{
		Event: EventQuizCompleted,
		Data: QuizUpdatePayload{
			QuizID: quizID,
		},
	}

	mu.RLock()
	conns := rooms[room]
	mu.RUnlock()

	for c := range conns {
		_ = c.WriteJSON(pl)
	}
}

func BroadcastQuizOCRDone(quizID int64, text string) {
	room := string(RoomQuiz) + "." + strconv.FormatInt(quizID, 10)

	pl := PayloadEvent{
		Event: EventQuizOCRDone,
		Data: QuizUpdatePayload{
			QuizID:  quizID,
			OCRText: text,
		},
	}

	mu.RLock()
	conns := rooms[room]
	mu.RUnlock()

	for c := range conns {
		_ = c.WriteJSON(pl)
	}
}

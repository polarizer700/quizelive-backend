package wsgame

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	gameapp "github.com/polarizer700/quizelive-backend/internal/app/game"
	"github.com/polarizer700/quizelive-backend/internal/config"
	"github.com/polarizer700/quizelive-backend/internal/domain/game"
)

type Server struct {
	hub      *gameapp.Hub
	patterns []string
}

func NewServer(hub *gameapp.Hub, cfg *config.Config) *Server {
	return &Server{hub: hub, patterns: originPatterns(cfg)}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /ws", s.serve)
	return mux
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: s.patterns})
	if err != nil {
		slog.Warn("websocket rejected", "category", "ws", "error", err)
		return
	}

	socket := newSocket(conn)
	defer socket.close()

	ctx := r.Context()
	go socket.write(ctx)

	var code string
	var joined bool
	for {
		_, body, err := conn.Read(ctx)
		if err != nil {
			if code != "" {
				s.hub.Leave(code, socket.ID())
			}
			return
		}

		command, kind, err := decode(body)
		if err != nil {
			socket.Send(game.Envelope{Error: &game.GameError{Code: "BAD_MESSAGE", Message: "непонятное сообщение"}})
			continue
		}

		command.ConnID = socket.ID()
		switch kind {
		case game.CommandHost:
			if joined {
				continue
			}

			s.hub.Create(ctx, command.Text, socket)
			code = waitCode(socket)
			joined = code != ""
		case game.CommandJoin:
			s.hub.Join(command.QuestionID, command.Nickname, command.PlayerToken, socket)
			code = command.QuestionID
			joined = true
		case game.CommandStart, game.CommandAnswer, game.CommandNext:
			if code == "" {
				socket.Send(game.Envelope{Error: &game.GameError{Code: "NO_ROOM", Message: "сначала откройте комнату"}})
				continue
			}
			s.hub.Command(code, command)
		default:
			socket.Send(game.Envelope{Error: &game.GameError{Code: "BAD_MESSAGE", Message: "непонятное сообщение"}})
		}
	}
}

func waitCode(socket *socket) string {
	deadline := time.After(2 * time.Second)
	for {
		select {
		case code := <-socket.codes:
			return code
		case <-deadline:
			return ""
		}
	}
}

type socket struct {
	id     string
	conn   *websocket.Conn
	out    chan []byte
	codes  chan string
	closed chan struct{}
	once   sync.Once
}

func newSocket(conn *websocket.Conn) *socket {
	return &socket{
		id:     uuid.NewString(),
		conn:   conn,
		out:    make(chan []byte, 32),
		codes:  make(chan string, 1),
		closed: make(chan struct{}),
	}
}

func (s *socket) ID() string { return s.id }

func (s *socket) Send(msg game.Envelope) {
	if msg.RoomState != nil && msg.RoomState.Code != "" {
		select {
		case s.codes <- msg.RoomState.Code:
		default:
		}
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return
	}

	select {
	case <-s.closed:
	case s.out <- body:
	default:
	}
}

func (s *socket) write(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.closed:
			return
		case body := <-s.out:
			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := s.conn.Write(writeCtx, websocket.MessageText, body)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func (s *socket) close() {
	s.once.Do(func() {
		close(s.closed)
		_ = s.conn.Close(websocket.StatusNormalClosure, "")
	})
}

type clientEnvelope struct {
	CreateRoom   *createRoom   `json:"create_room"`
	JoinRoom     *joinRoom     `json:"join_room"`
	StartGame    *emptyCommand `json:"start_game"`
	SubmitAnswer *submitAnswer `json:"submit_answer"`
	NextStep     *emptyCommand `json:"next_step"`
}

type emptyCommand struct{}

type createRoom struct {
	HostToken string `json:"host_token"`
}

type joinRoom struct {
	Code        string `json:"code"`
	Nickname    string `json:"nickname"`
	PlayerToken string `json:"player_token"`
}

type submitAnswer struct {
	QuestionID string   `json:"question_id"`
	OptionIDs  []string `json:"option_ids"`
	Text       string   `json:"text"`
}

func decode(body []byte) (game.Command, game.CommandKind, error) {
	var envelope clientEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return game.Command{}, "", err
	}

	filled := 0
	if envelope.CreateRoom != nil {
		filled++
	}
	if envelope.JoinRoom != nil {
		filled++
	}
	if envelope.StartGame != nil {
		filled++
	}
	if envelope.SubmitAnswer != nil {
		filled++
	}
	if envelope.NextStep != nil {
		filled++
	}
	if filled != 1 {
		return game.Command{}, "", errors.New("one command")
	}

	switch {
	case envelope.CreateRoom != nil:
		return game.Command{Kind: game.CommandHost, Text: envelope.CreateRoom.HostToken}, game.CommandHost, nil
	case envelope.JoinRoom != nil:
		return game.Command{
			Kind:        game.CommandJoin,
			QuestionID:  envelope.JoinRoom.Code,
			Nickname:    envelope.JoinRoom.Nickname,
			PlayerToken: envelope.JoinRoom.PlayerToken,
		}, game.CommandJoin, nil
	case envelope.StartGame != nil:
		return game.Command{Kind: game.CommandStart}, game.CommandStart, nil
	case envelope.SubmitAnswer != nil:
		return game.Command{
			Kind:       game.CommandAnswer,
			QuestionID: envelope.SubmitAnswer.QuestionID,
			OptionIDs:  envelope.SubmitAnswer.OptionIDs,
			Text:       envelope.SubmitAnswer.Text,
		}, game.CommandAnswer, nil
	default:
		return game.Command{Kind: game.CommandNext}, game.CommandNext, nil
	}
}

func originPatterns(cfg *config.Config) []string {
	patterns := make([]string, 0, len(cfg.CORSOrigins())+2)
	if cfg.AppEnv == "local" {
		patterns = append(patterns, "localhost:*", "127.0.0.1:*")
	}

	for _, origin := range cfg.CORSOrigins() {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host == "" {
			continue
		}

		patterns = append(patterns, parsed.Host)
	}

	return patterns
}

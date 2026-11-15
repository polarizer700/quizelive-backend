package gameapp

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/polarizer700/quizelive-backend/internal/domain"
	"github.com/polarizer700/quizelive-backend/internal/domain/game"
	"github.com/polarizer700/quizelive-backend/internal/domain/repository"
)

type Subscriber interface {
	ID() string
	Send(game.Envelope)
}

type inbound struct {
	sub Subscriber
	cmd game.Command
}

type Hub struct {
	loader   *Loader
	sessions repository.GameSessionRepository
	mu       sync.Mutex
	rooms    map[string]*mailbox
	byHost   map[string]*mailbox
	byResume map[string]*mailbox
}

type mailbox struct {
	code     string
	in       chan inbound
	finished bool
}

func NewHub(loader *Loader, sessions repository.GameSessionRepository) *Hub {
	return &Hub{
		loader:   loader,
		sessions: sessions,
		rooms:    map[string]*mailbox{},
		byHost:   map[string]*mailbox{},
		byResume: map[string]*mailbox{},
	}
}

func (h *Hub) Create(ctx context.Context, hostToken string, sub Subscriber) {
	h.mu.Lock()
	if existing := h.byResume[hostToken]; existing != nil {
		finished := existing.finished
		h.mu.Unlock()
		if finished {
			sub.Send(game.Envelope{Error: &game.GameError{
				Code:    "ROOM_FINISHED",
				Message: "игра уже закончилась — выпустите новый токен ведущего",
			}})
			return
		}
		existing.in <- inbound{
			sub: sub,
			cmd: game.Command{Kind: game.CommandHost, ConnID: sub.ID(), Now: time.Now()},
		}
		return
	}
	h.mu.Unlock()

	hostID, quizID, err := h.loader.Identify(ctx, hostToken)
	if err != nil {
		sub.Send(errorFrom(err))
		return
	}

	key := hostID.String() + ":" + quizID.String()
	h.mu.Lock()
	// Новый host-JWT всегда открывает новую комнату. Переподключение экрана идёт через resume.
	if existing := h.byHost[key]; existing != nil {
		h.detachLocked(existing)
		delete(h.byHost, key)
		existing.finished = true
		// Просим старую комнату предупредить игроков и завершиться,
		// чтобы не висеть зомби с тем же кодом
		select {
		case existing.in <- inbound{cmd: game.Command{Kind: game.CommandClose, Now: time.Now()}}:
		default:
		}
	}

	box := &mailbox{in: make(chan inbound, 64)}
	h.byHost[key] = box
	h.mu.Unlock()

	var catalog Catalog
	var code string
	for range 8 {
		code, err = newCode()
		if err != nil {
			break
		}

		catalog, err = h.loader.Open(ctx, hostToken, code)
		if err == nil || !errors.Is(err, domain.ErrCodeTaken) {
			break
		}
	}

	if err != nil {
		h.mu.Lock()
		delete(h.byHost, key)
		h.mu.Unlock()
		sub.Send(errorFrom(err))
		return
	}

	box.code = code
	h.mu.Lock()
	h.rooms[code] = box
	h.mu.Unlock()

	room := game.NewRoom(code, catalog.SessionID, catalog.HostID, catalog.Questions, catalog.GameType, catalog.BreakAfterRound)
	h.mu.Lock()
	h.byResume[room.Resume] = box
	h.mu.Unlock()
	go h.run(box, room)
	box.in <- inbound{
		sub: sub,
		cmd: game.Command{Kind: game.CommandHost, ConnID: sub.ID(), Now: time.Now()},
	}
}

func (h *Hub) Join(code, nickname, playerToken string, sub Subscriber) {
	box := h.box(code)
	if box == nil {
		sub.Send(game.Envelope{Error: &game.GameError{Code: "ROOM_NOT_FOUND", Message: "комната не найдена"}})
		return
	}

	box.in <- inbound{
		sub: sub,
		cmd: game.Command{
			Kind:        game.CommandJoin,
			ConnID:      sub.ID(),
			Nickname:    nickname,
			PlayerToken: playerToken,
			Now:         time.Now(),
		},
	}
}

func (h *Hub) Command(code string, cmd game.Command) {
	box := h.box(code)
	if box == nil {
		return
	}

	cmd.Now = time.Now()
	box.in <- inbound{cmd: cmd}
}

func (h *Hub) Leave(code, connID string) {
	box := h.box(code)
	if box == nil {
		return
	}

	box.in <- inbound{cmd: game.Command{Kind: game.CommandLeave, ConnID: connID, Now: time.Now()}}
}

func (h *Hub) box(code string) *mailbox {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.rooms[code]
}

func (h *Hub) detachLocked(box *mailbox) {
	for token, candidate := range h.byResume {
		if candidate == box {
			delete(h.byResume, token)
		}
	}
	if box.code != "" {
		delete(h.rooms, box.code)
	}
}

func (h *Hub) run(box *mailbox, room *game.Room) {
	subs := map[string]Subscriber{}
	var timer *time.Timer

	dispatch := func(deliveries []game.Delivery) {
		for _, delivery := range deliveries {
			if delivery.Disarm && timer != nil {
				stopTimer(timer)
			}

			if delivery.ArmTimer {
				timer = armTimer(timer, delivery.Deadline)
			}

			emit(subs, delivery)
			if delivery.Finished != nil {
				h.mu.Lock()
				box.finished = true
				h.mu.Unlock()
				h.persist(room, delivery.Finished)
			}
		}
	}

	var ticks <-chan time.Time
	for {
		if timer != nil {
			ticks = timer.C
		} else {
			ticks = nil
		}

		select {
		case msg := <-box.in:
			if msg.sub != nil {
				subs[msg.sub.ID()] = msg.sub
			}

			if msg.cmd.Kind == game.CommandLeave {
				delete(subs, msg.cmd.ConnID)
			}

			dispatch(room.Apply(msg.cmd))
			if msg.cmd.Kind == game.CommandClose {
				return
			}
		case now := <-ticks:
			dispatch(room.Apply(game.Command{Kind: game.CommandTick, Now: now}))
		}
	}
}

func emit(subs map[string]Subscriber, delivery game.Delivery) {
	if delivery.Msg == (game.Envelope{}) {
		return
	}

	if delivery.All {
		for _, sub := range subs {
			sub.Send(delivery.Msg)
		}

		return
	}

	if sub := subs[delivery.ConnID]; sub != nil {
		sub.Send(delivery.Msg)
	}
}

func (h *Hub) persist(room *game.Room, results []game.Result) {
	rows := make([]repository.GameResult, 0, len(results))
	for _, result := range results {
		rows = append(rows, repository.GameResult{
			Nickname: result.Nickname,
			Score:    result.Score,
			Rank:     result.Rank,
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.sessions.Finish(ctx, room.SessionID, time.Now(), rows); err != nil {
		slog.Error("save game results", "category", "game", "code", room.Code, "error", err)
	}
}

func armTimer(timer *time.Timer, deadline time.Time) *time.Timer {
	wait := time.Until(deadline)
	if wait < 0 {
		wait = 0
	}

	if timer == nil {
		return time.NewTimer(wait)
	}

	stopTimer(timer)
	timer.Reset(wait)

	return timer
}

func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func newCode() (string, error) {
	var buf [2]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}

	value := int(buf[0])<<8 | int(buf[1])

	return fmt.Sprintf("%04d", value%10000), nil
}

func errorFrom(err error) game.Envelope {
	switch {
	case errors.Is(err, domain.ErrUnauthorized):
		return game.Envelope{Error: &game.GameError{Code: "UNAUTHORIZED", Message: "токен ведущего недействителен"}}
	case errors.Is(err, domain.ErrNotFound):
		return game.Envelope{Error: &game.GameError{Code: "QUIZ_NOT_FOUND", Message: "викторина не найдена"}}
	case errors.Is(err, domain.ErrConflict):
		var conflict *domain.ConflictError
		message := "комната не открылась"
		if errors.As(err, &conflict) && conflict.Message != "" {
			message = conflict.Message
		}

		return game.Envelope{Error: &game.GameError{Code: "CONFLICT", Message: message}}
	default:
		return game.Envelope{Error: &game.GameError{Code: "INTERNAL", Message: "не удалось открыть комнату"}}
	}
}

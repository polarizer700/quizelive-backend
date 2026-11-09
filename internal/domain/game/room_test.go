package game

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/polarizer700/quizelive-backend/internal/domain/entity"
)

func TestRoomHidesAnswersUntilReveal(t *testing.T) {
	room := sampleRoom()
	now := time.Date(2026, 11, 8, 20, 0, 0, 0, time.UTC)
	host := room.Apply(Command{Kind: CommandHost, ConnID: "tv", Now: now})
	if host[0].Msg.HostResume == nil || host[0].Msg.HostResume.Token == "" {
		t.Fatalf("host did not receive a resume token: %+v", host[0].Msg)
	}
	if host[1].Msg.RoomState == nil || host[1].Msg.RoomState.Phase != PhaseLobby {
		t.Fatalf("host did not see the lobby: %+v", host[1].Msg)
	}

	joined := room.Apply(Command{Kind: CommandJoin, ConnID: "phone", Nickname: "Аня", Now: now})
	if joined[0].Msg.PlayerSession == nil || joined[0].Msg.PlayerSession.PlayerToken == "" {
		t.Fatal("player did not receive a token")
	}

	started := room.Apply(Command{Kind: CommandStart, ConnID: "tv", Now: now})
	var question *QuestionStarted
	for _, delivery := range started {
		if delivery.Msg.QuestionStarted != nil {
			question = delivery.Msg.QuestionStarted
		}
	}

	if question == nil || question.Options[0].Text != "Париж" {
		t.Fatalf("question was not announced: %+v", question)
	}

	if len(question.Options) != 2 {
		t.Fatalf("options leaked or dropped: %+v", question.Options)
	}

	fast := room.Apply(Command{
		Kind:       CommandAnswer,
		ConnID:     "phone",
		QuestionID: question.QuestionID,
		OptionIDs:  []string{room.Questions[0].Options[0].ID},
		Now:        now,
	})
	if fast[0].Msg.AnswerAccepted == nil {
		t.Fatalf("answer was not accepted: %+v", fast[0].Msg)
	}

	revealed := room.Apply(Command{Kind: CommandNext, ConnID: "tv", Now: now.Add(2 * time.Second)})
	var reveal *QuestionRevealed
	for _, delivery := range revealed {
		if delivery.Msg.QuestionRevealed != nil {
			reveal = delivery.Msg.QuestionRevealed
		}
	}

	if reveal == nil || len(reveal.CorrectOptionIDs) != 1 || reveal.CorrectOptionIDs[0] != room.Questions[0].Options[0].ID {
		t.Fatalf("reveal = %+v", reveal)
	}

	if room.Players[0].Score != 100 {
		t.Fatalf("fast correct score = %d", room.Players[0].Score)
	}
}

func TestTextAnswerAndReconnect(t *testing.T) {
	room := sampleRoom()
	now := time.Now()
	room.Questions = []Question{{
		ID:      "q-text",
		Type:    entity.QuestionTypeText,
		Text:    "Два плюс два",
		Limit:   10 * time.Second,
		Points:  10,
		Answers: []string{entity.NormalizeTextAnswer("4")},
	}}
	room.Apply(Command{Kind: CommandHost, ConnID: "tv", Now: now})
	joined := room.Apply(Command{Kind: CommandJoin, ConnID: "phone", Nickname: "Марк", Now: now})
	token := joined[0].Msg.PlayerSession.PlayerToken
	room.Apply(Command{Kind: CommandStart, ConnID: "tv", Now: now})
	accepted := room.Apply(Command{
		Kind:       CommandAnswer,
		ConnID:     "phone",
		QuestionID: "q-text",
		Text:       "  4 ",
		Now:        now.Add(5 * time.Second),
	})
	if accepted[0].Msg.AnswerAccepted == nil {
		t.Fatalf("text answer rejected: %+v", accepted[0].Msg)
	}

	room.Apply(Command{Kind: CommandLeave, ConnID: "phone", Now: now})
	again := room.Apply(Command{Kind: CommandJoin, ConnID: "phone-2", PlayerToken: token, Now: now})
	if again[0].Msg.PlayerSession.Player.Nickname != "Марк" {
		t.Fatalf("reconnect created another player: %+v", again[0].Msg.PlayerSession)
	}

	if len(room.Players) != 1 || !room.Players[0].Connected {
		t.Fatalf("players = %+v", room.Players)
	}
}

func TestPacingSkipsBoardBetweenCheckpoints(t *testing.T) {
	now := time.Date(2026, 11, 9, 21, 0, 0, 0, time.UTC)
	questions := make([]Question, 4)
	for i := range questions {
		questions[i] = Question{
			ID:     uuid.NewString(),
			Type:   entity.QuestionTypeSingle,
			Text:   "Q",
			Limit:  time.Second,
			Points: 10,
			Options: []Option{
				{ID: "a", Text: "A", IsCorrect: true},
				{ID: "b", Text: "B"},
			},
		}
	}
	room := NewRoom("1111", uuid.New(), uuid.New(), questions)
	room.Apply(Command{Kind: CommandHost, ConnID: "tv", Now: now})
	room.Apply(Command{Kind: CommandJoin, ConnID: "phone", Nickname: "Аня", Now: now})
	room.Apply(Command{Kind: CommandStart, ConnID: "tv", Now: now})

	// After Q1 of 4: half is 2, so no mid-board — jump to Q2.
	room.Apply(Command{Kind: CommandNext, ConnID: "tv", Now: now.Add(time.Second)}) // reveal
	after := room.Apply(Command{Kind: CommandNext, ConnID: "tv", Now: now.Add(5 * time.Second)})
	if room.Phase != PhaseQuestion || room.Index != 1 {
		t.Fatalf("expected Q2 after reveal, phase=%s index=%d deliveries=%d", room.Phase, room.Index, len(after))
	}

	// After Q2 of 4: mid-board.
	room.Apply(Command{Kind: CommandNext, ConnID: "tv", Now: now.Add(6 * time.Second)}) // reveal
	board := room.Apply(Command{Kind: CommandNext, ConnID: "tv", Now: now.Add(10 * time.Second)})
	if room.Phase != PhaseLeaderboard {
		t.Fatalf("expected mid-board, phase=%s deliveries=%d", room.Phase, len(board))
	}
}

func TestSpeedBonusShrinks(t *testing.T) {
	limit := 20 * time.Second
	if got := Points(100, true, 0, limit); got != 100 {
		t.Fatalf("instant = %d", got)
	}

	if got := Points(100, true, 10*time.Second, limit); got != 75 {
		t.Fatalf("halfway = %d", got)
	}

	if got := Points(100, true, limit, limit); got != 50 {
		t.Fatalf("last moment = %d", got)
	}

	if got := Points(100, false, 0, limit); got != 0 {
		t.Fatalf("wrong = %d", got)
	}
}

func sampleRoom() *Room {
	return NewRoom("4821", uuid.New(), uuid.New(), []Question{{
		ID:     "q1",
		Type:   entity.QuestionTypeSingle,
		Text:   "Столица Франции?",
		Limit:  20 * time.Second,
		Points: 100,
		Options: []Option{
			{ID: "paris", Text: "Париж", IsCorrect: true},
			{ID: "lyon", Text: "Лион"},
		},
	}})
}

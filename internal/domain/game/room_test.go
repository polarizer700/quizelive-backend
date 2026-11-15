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
	room := NewRoom("1111", uuid.New(), uuid.New(), questions, entity.GameTypeQuiz, 0)
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

func TestAnswersProgressAndSoftClose(t *testing.T) {
	room := sampleRoom()
	now := time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)
	room.Apply(Command{Kind: CommandHost, ConnID: "tv", Now: now})
	room.Apply(Command{Kind: CommandJoin, ConnID: "phone", Nickname: "Аня", Now: now})
	room.Apply(Command{Kind: CommandJoin, ConnID: "phone-2", Nickname: "Боря", Now: now})
	room.Apply(Command{Kind: CommandStart, ConnID: "tv", Now: now})
	original := room.deadline

	first := room.Apply(Command{
		Kind:       CommandAnswer,
		ConnID:     "phone",
		QuestionID: "q1",
		OptionIDs:  []string{"paris"},
		Now:        now.Add(time.Second),
	})
	var progress *AnswersProgress
	for _, delivery := range first {
		if delivery.Msg.AnswersProgress != nil {
			progress = delivery.Msg.AnswersProgress
		}
	}
	if progress == nil || progress.Answered != 1 || progress.Total != 2 {
		t.Fatalf("progress after first = %+v", progress)
	}
	if !room.deadline.Equal(original) {
		t.Fatalf("deadline moved too early: %v vs %v", room.deadline, original)
	}

	second := room.Apply(Command{
		Kind:       CommandAnswer,
		ConnID:     "phone-2",
		QuestionID: "q1",
		OptionIDs:  []string{"lyon"},
		Now:        now.Add(4 * time.Second),
	})
	for _, delivery := range second {
		if delivery.Msg.AnswersProgress != nil {
			progress = delivery.Msg.AnswersProgress
		}
	}
	if progress == nil || progress.Answered != 2 || progress.Total != 2 {
		t.Fatalf("progress after all = %+v", progress)
	}
	want := now.Add(6 * time.Second)
	if !room.deadline.Equal(want) {
		t.Fatalf("soft deadline = %v, want %v", room.deadline, want)
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
	}}, entity.GameTypeQuiz, 0)
}

func TestPubQuizBreakAfterRound(t *testing.T) {
	now := time.Date(2026, 11, 15, 20, 0, 0, 0, time.UTC)
	questions := []Question{
		{ID: "q1", Type: entity.QuestionTypeSingle, Text: "Блиц 1", Limit: time.Second, Points: 10, Category: "Блиц",
			Options: []Option{{ID: "a", Text: "A", IsCorrect: true}, {ID: "b", Text: "B"}}},
		{ID: "q2", Type: entity.QuestionTypeSingle, Text: "Блиц 2", Limit: time.Second, Points: 10, Category: "Блиц",
			Options: []Option{{ID: "a", Text: "A", IsCorrect: true}, {ID: "b", Text: "B"}}},
		{ID: "q3", Type: entity.QuestionTypeSingle, Text: "Хардкор 1", Limit: time.Second, Points: 10, Category: "Хардкор",
			Options: []Option{{ID: "a", Text: "A", IsCorrect: true}, {ID: "b", Text: "B"}}},
	}
	room := NewRoom("7777", uuid.New(), uuid.New(), questions, entity.GameTypePubQuiz, 1)
	room.Apply(Command{Kind: CommandHost, ConnID: "tv", Now: now})
	room.Apply(Command{Kind: CommandJoin, ConnID: "phone", Nickname: "Аня", Now: now})

	started := room.Apply(Command{Kind: CommandStart, ConnID: "tv", Now: now})
	var first *QuestionStarted
	for _, delivery := range started {
		if delivery.Msg.QuestionStarted != nil {
			first = delivery.Msg.QuestionStarted
		}
	}
	if first == nil || first.Category != "Блиц" || first.Round != 1 || first.RoundTotal != 2 {
		t.Fatalf("round info = %+v", first)
	}

	// Q1 -> reveal -> мид-борд (половина из 3 вопросов) -> Q2 (ещё не конец раунда, перерыва нет).
	room.Apply(Command{Kind: CommandNext, ConnID: "tv", Now: now.Add(2 * time.Second)})
	room.Apply(Command{Kind: CommandNext, ConnID: "tv", Now: now.Add(3 * time.Second)})
	if room.Phase != PhaseLeaderboard {
		t.Fatalf("phase after first reveal = %s", room.Phase)
	}
	room.Apply(Command{Kind: CommandNext, ConnID: "tv", Now: now.Add(4 * time.Second)})
	if room.Phase != PhaseQuestion {
		t.Fatalf("phase after board = %s", room.Phase)
	}

	// Q2 -> reveal -> конец раунда 1: перерыв.
	room.Apply(Command{Kind: CommandNext, ConnID: "tv", Now: now.Add(5 * time.Second)})
	if room.Phase != PhaseReveal {
		t.Fatalf("phase before break = %s", room.Phase)
	}
	breakOut := room.Apply(Command{Kind: CommandNext, ConnID: "tv", Now: now.Add(5 * time.Second)})
	if room.Phase != PhaseBreak {
		t.Fatalf("phase = %s, want break", room.Phase)
	}
	sawState := false
	for _, delivery := range breakOut {
		if delivery.Msg.RoomState != nil && delivery.Msg.RoomState.Phase == PhaseBreak {
			sawState = true
		}
	}
	if !sawState {
		t.Fatal("break phase was not announced")
	}

	// Продолжаем после перерыва — следующий вопрос из раунда 2.
	continued := room.Apply(Command{Kind: CommandNext, ConnID: "tv", Now: now.Add(6 * time.Second)})
	var next *QuestionStarted
	for _, delivery := range continued {
		if delivery.Msg.QuestionStarted != nil {
			next = delivery.Msg.QuestionStarted
		}
	}
	if next == nil || next.Category != "Хардкор" || next.Round != 2 {
		t.Fatalf("question after break = %+v", next)
	}

	// Тик без ведущего не должен выводить из перерыва.
	room2 := NewRoom("8888", uuid.New(), uuid.New(), questions, entity.GameTypePubQuiz, 1)
	room2.Apply(Command{Kind: CommandHost, ConnID: "tv", Now: now})
	room2.Apply(Command{Kind: CommandJoin, ConnID: "phone", Nickname: "Аня", Now: now})
	room2.Apply(Command{Kind: CommandStart, ConnID: "tv", Now: now})
	room2.Phase = PhaseBreak
	ticked := room2.Apply(Command{Kind: CommandTick, ConnID: "", Now: now.Add(time.Hour)})
	if room2.Phase != PhaseBreak || len(ticked) != 0 {
		t.Fatalf("tick in break = %s, %v", room2.Phase, ticked)
	}
}

package game

import (
	"crypto/rand"
	"encoding/base64"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/polarizer700/quizelive-backend/internal/domain/entity"
)

const (
	PhaseLobby       = "lobby"
	PhaseQuestion    = "question"
	PhaseReveal      = "reveal"
	PhaseLeaderboard = "leaderboard"
	PhaseBreak       = "break"
	PhaseFinished    = "finished"
	maxPlayers       = 100
	RevealHold       = 4 * time.Second
	BoardHold        = 5 * time.Second
	AllAnsweredMin   = 3 * time.Second
	AllAnsweredGrace = 2 * time.Second
)

type Phase string

type Option struct {
	ID        string
	Text      string
	IsCorrect bool
}

type Question struct {
	ID       string
	Type     entity.QuestionType
	Text     string
	Limit    time.Duration
	Points   int
	Category string
	Options  []Option
	Answers  []string
	ImageURL string
}

type Result struct {
	Nickname string
	Score    int
	Rank     int
}

type CommandKind string

const (
	CommandHost   CommandKind = "host"
	CommandJoin   CommandKind = "join"
	CommandLeave  CommandKind = "leave"
	CommandStart  CommandKind = "start"
	CommandAnswer CommandKind = "answer"
	CommandNext   CommandKind = "next"
	CommandTick   CommandKind = "tick"
)

type Command struct {
	Kind        CommandKind
	ConnID      string
	Nickname    string
	PlayerToken string
	QuestionID  string
	OptionIDs   []string
	Text        string
	Now         time.Time
}

// Delivery описывает, кому отправить кадр. All значит всем текущим сокетам комнаты.
type Delivery struct {
	All      bool
	ConnID   string
	Msg      Envelope
	ArmTimer bool
	Disarm   bool
	Deadline time.Time
	Finished []Result
}

type player struct {
	ID        string
	Nickname  string
	Score     int
	Connected bool
	ConnID    string
	Token     string
}

type submission struct {
	OptionIDs []string
	Text      string
	Elapsed   time.Duration
}

// Room хранит состояние одной комнаты. Вызывать методы можно только из горутины этой комнаты.
type Room struct {
	Code            string
	SessionID       uuid.UUID
	HostID          uuid.UUID
	HostConn        string
	Resume          string
	Questions       []Question
	GameType        entity.QuizGameType
	BreakAfterRound int
	roundOf         []int
	roundCount      int
	Index           int
	Phase           Phase
	Players         []*player
	byNick          map[string]*player
	byToken         map[string]*player
	answers         map[string]submission
	started         time.Time
	deadline        time.Time
	finishedSet     bool
}

func NewRoom(code string, sessionID, hostID uuid.UUID, questions []Question, gameType entity.QuizGameType, breakAfterRound int) *Room {
	resume, err := newToken()
	if err != nil {
		resume = code + "-host"
	}

	roundOf, roundCount := roundLayout(questions)

	return &Room{
		Code:            code,
		SessionID:       sessionID,
		HostID:          hostID,
		Resume:          resume,
		Questions:       questions,
		GameType:        gameType,
		BreakAfterRound: breakAfterRound,
		roundOf:         roundOf,
		roundCount:      roundCount,
		Phase:           PhaseLobby,
		byNick:          map[string]*player{},
		byToken:         map[string]*player{},
		answers:         map[string]submission{},
	}
}

// roundLayout разбивает вопросы на раунды по последовательным группам
// одинаковых категорий и возвращает номер раунда (с 1) для каждого вопроса
// и общее число раундов.
func roundLayout(questions []Question) ([]int, int) {
	roundOf := make([]int, len(questions))
	round := 0
	last := ""
	for i, question := range questions {
		if i == 0 || question.Category != last {
			round++
			last = question.Category
		}

		roundOf[i] = round
	}

	return roundOf, round
}

func (r *Room) Apply(cmd Command) []Delivery {
	switch cmd.Kind {
	case CommandHost:
		return r.attachHost(cmd)
	case CommandJoin:
		return r.join(cmd)
	case CommandLeave:
		return r.leave(cmd)
	case CommandStart:
		return r.start(cmd)
	case CommandAnswer:
		return r.answer(cmd)
	case CommandNext:
		return r.next(cmd)
	case CommandTick:
		return r.tick(cmd)
	default:
		return []Delivery{{ConnID: cmd.ConnID, Msg: errorEnvelope("BAD_COMMAND", "неизвестная команда")}}
	}
}

func (r *Room) attachHost(cmd Command) []Delivery {
	r.HostConn = cmd.ConnID

	return append([]Delivery{
		{ConnID: cmd.ConnID, Msg: Envelope{HostResume: &HostResume{Token: r.Resume}}},
		{ConnID: cmd.ConnID, Msg: r.stateEnvelope()},
	}, r.replay(cmd.ConnID)...)
}

func (r *Room) join(cmd Command) []Delivery {
	if r.Phase == PhaseFinished {
		return []Delivery{{ConnID: cmd.ConnID, Msg: errorEnvelope("ROOM_CLOSED", "игра уже закончилась")}}
	}

	if cmd.PlayerToken != "" {
		if existing := r.byToken[cmd.PlayerToken]; existing != nil {
			existing.ConnID = cmd.ConnID
			existing.Connected = true
			out := []Delivery{{
				ConnID: cmd.ConnID,
				Msg: Envelope{PlayerSession: &PlayerSession{
					PlayerToken: existing.Token,
					Player:      existing.view(),
				}},
			}}
			out = append(out, Delivery{ConnID: cmd.ConnID, Msg: r.stateEnvelope()})
			out = append(out, r.replay(cmd.ConnID)...)
			out = append(out, Delivery{All: true, Msg: r.stateEnvelope()})

			return out
		}
	}

	nick := strings.TrimSpace(cmd.Nickname)
	if n := utf8.RuneCountInString(nick); n < 1 || n > 20 {
		return []Delivery{{ConnID: cmd.ConnID, Msg: errorEnvelope("BAD_NICK", "имя — от 1 до 20 символов")}}
	}

	if len(r.Players) >= maxPlayers {
		return []Delivery{{ConnID: cmd.ConnID, Msg: errorEnvelope("ROOM_FULL", "в комнате больше нет мест")}}
	}

	key := strings.ToLower(nick)
	if _, taken := r.byNick[key]; taken {
		return []Delivery{{ConnID: cmd.ConnID, Msg: errorEnvelope("NICK_TAKEN", "такое имя уже в комнате")}}
	}

	token, err := newToken()
	if err != nil {
		return []Delivery{{ConnID: cmd.ConnID, Msg: errorEnvelope("INTERNAL", "не удалось выдать токен")}}
	}

	created := &player{
		ID:        uuid.NewString(),
		Nickname:  nick,
		Connected: true,
		ConnID:    cmd.ConnID,
		Token:     token,
	}
	r.Players = append(r.Players, created)
	r.byNick[key] = created
	r.byToken[token] = created

	out := []Delivery{{
		ConnID: cmd.ConnID,
		Msg: Envelope{PlayerSession: &PlayerSession{
			PlayerToken: token,
			Player:      created.view(),
		}},
	}}
	out = append(out, Delivery{ConnID: cmd.ConnID, Msg: r.stateEnvelope()})
	out = append(out, r.replay(cmd.ConnID)...)
	out = append(out, Delivery{All: true, Msg: Envelope{PlayerJoined: &PlayerJoined{Player: created.view()}}})
	out = append(out, Delivery{All: true, Msg: r.stateEnvelope()})

	return out
}

func (r *Room) leave(cmd Command) []Delivery {
	if cmd.ConnID == r.HostConn {
		r.HostConn = ""
		return []Delivery{{All: true, Msg: r.stateEnvelope()}}
	}

	for _, item := range r.Players {
		if item.ConnID == cmd.ConnID {
			item.Connected = false
			item.ConnID = ""
			return []Delivery{{All: true, Msg: r.stateEnvelope()}}
		}
	}

	return nil
}

func (r *Room) start(cmd Command) []Delivery {
	if cmd.ConnID != r.HostConn {
		return []Delivery{{ConnID: cmd.ConnID, Msg: errorEnvelope("NOT_HOST", "начать игру может только ведущий")}}
	}

	if r.Phase != PhaseLobby {
		return []Delivery{{ConnID: cmd.ConnID, Msg: errorEnvelope("BAD_PHASE", "игра уже идёт")}}
	}

	if len(r.Players) == 0 {
		return []Delivery{{ConnID: cmd.ConnID, Msg: errorEnvelope("NO_PLAYERS", "дождитесь хотя бы одного игрока")}}
	}

	if len(r.Questions) == 0 {
		return []Delivery{{ConnID: cmd.ConnID, Msg: errorEnvelope("QUIZ_EMPTY", "в викторине нет вопросов")}}
	}

	return r.openQuestion(cmd.Now)
}

func (r *Room) answer(cmd Command) []Delivery {
	if r.Phase != PhaseQuestion {
		return []Delivery{{ConnID: cmd.ConnID, Msg: errorEnvelope("BAD_PHASE", "сейчас нельзя отвечать")}}
	}

	current := r.Questions[r.Index]
	if cmd.QuestionID != current.ID {
		return []Delivery{{ConnID: cmd.ConnID, Msg: errorEnvelope("BAD_QUESTION", "этот вопрос уже закрыт")}}
	}

	if !cmd.Now.Before(r.deadline) {
		return []Delivery{{ConnID: cmd.ConnID, Msg: errorEnvelope("LATE", "время вышло")}}
	}

	item := r.playerByConn(cmd.ConnID)
	if item == nil {
		return []Delivery{{ConnID: cmd.ConnID, Msg: errorEnvelope("NOT_PLAYER", "сначала войдите в комнату")}}
	}

	if _, ok := r.answers[item.ID]; ok {
		return []Delivery{{ConnID: cmd.ConnID, Msg: errorEnvelope("ALREADY_ANSWERED", "ответ уже принят")}}
	}

	text := strings.TrimSpace(cmd.Text)
	if current.Type == entity.QuestionTypeText && text == "" {
		return []Delivery{{ConnID: cmd.ConnID, Msg: errorEnvelope("EMPTY_ANSWER", "впишите ответ")}}
	}

	if current.Type != entity.QuestionTypeText && len(cmd.OptionIDs) == 0 {
		return []Delivery{{ConnID: cmd.ConnID, Msg: errorEnvelope("EMPTY_ANSWER", "выберите вариант")}}
	}

	r.answers[item.ID] = submission{
		OptionIDs: slices.Clone(cmd.OptionIDs),
		Text:      text,
		Elapsed:   cmd.Now.Sub(r.started),
	}

	out := []Delivery{{
		ConnID: cmd.ConnID,
		Msg:    Envelope{AnswerAccepted: &AnswerAccepted{QuestionID: current.ID}},
	}}

	if r.allConnectedAnswered() {
		earliest := r.started.Add(AllAnsweredMin)
		soon := cmd.Now.Add(AllAnsweredGrace)
		next := earliest
		if soon.After(next) {
			next = soon
		}
		if next.Before(r.deadline) {
			r.deadline = next
		}
	}

	progress := r.answersProgress()
	out = append(out, Delivery{
		All:      true,
		Msg:      Envelope{AnswersProgress: &progress},
		ArmTimer: true,
		Deadline: r.deadline,
	})

	return out
}

func (r *Room) next(cmd Command) []Delivery {
	if cmd.ConnID != r.HostConn {
		return []Delivery{{ConnID: cmd.ConnID, Msg: errorEnvelope("NOT_HOST", "ход ведёт только ведущий")}}
	}

	switch r.Phase {
	case PhaseQuestion:
		return r.reveal(cmd.Now)
	case PhaseReveal:
		return r.afterReveal(cmd.Now)
	case PhaseLeaderboard:
		return r.afterBoard(cmd.Now)
	case PhaseBreak:
		return r.advance(cmd.Now)
	default:
		return []Delivery{{ConnID: cmd.ConnID, Msg: errorEnvelope("BAD_PHASE", "сейчас нечего переключать")}}
	}
}

func (r *Room) tick(cmd Command) []Delivery {
	if cmd.Now.Before(r.deadline) {
		return nil
	}

	switch r.Phase {
	case PhaseQuestion:
		return r.reveal(cmd.Now)
	case PhaseReveal:
		return r.afterReveal(cmd.Now)
	case PhaseLeaderboard:
		return r.afterBoard(cmd.Now)
	default:
		return nil
	}
}

func (r *Room) openQuestion(now time.Time) []Delivery {
	question := r.Questions[r.Index]
	r.Phase = PhaseQuestion
	r.answers = map[string]submission{}
	r.started = now
	r.deadline = now.Add(question.Limit)
	view := r.questionView(r.Index, r.deadline)

	return []Delivery{
		{All: true, Msg: r.stateEnvelope()},
		{All: true, Msg: Envelope{QuestionStarted: &view}, ArmTimer: true, Deadline: r.deadline},
	}
}

func (r *Room) reveal(now time.Time) []Delivery {
	question := r.Questions[r.Index]
	for _, item := range r.Players {
		submission, ok := r.answers[item.ID]
		if !ok {
			continue
		}

		item.Score += Points(question.Points, question.correct(submission), submission.Elapsed, question.Limit)
	}

	r.Phase = PhaseReveal
	r.deadline = now.Add(RevealHold)
	revealed := question.reveal()

	return []Delivery{
		{Disarm: true},
		{All: true, Msg: Envelope{QuestionRevealed: &revealed}},
		{All: true, Msg: r.stateEnvelope(), ArmTimer: true, Deadline: r.deadline},
	}
}

func (r *Room) afterReveal(now time.Time) []Delivery {
	total := len(r.Questions)
	if r.shouldBreak(total) {
		return r.enterBreak()
	}

	questionNumber := r.Index + 1
	if r.Index >= total-1 || ShouldShowMidBoard(questionNumber, total) {
		return r.showBoard(now)
	}

	return r.advance(now)
}

// shouldBreak сообщает, нужен ли перерыв после текущего вопроса:
// только в квизе, после последнего вопроса раунда BreakAfterRound, если впереди ещё есть вопросы.
func (r *Room) shouldBreak(total int) bool {
	if r.GameType != entity.GameTypePubQuiz || r.BreakAfterRound <= 0 || r.Index >= total-1 {
		return false
	}

	if r.roundOf[r.Index] != r.BreakAfterRound {
		return false
	}

	return r.roundOf[r.Index+1] != r.BreakAfterRound
}

func (r *Room) enterBreak() []Delivery {
	r.Phase = PhaseBreak

	return []Delivery{
		{Disarm: true},
		{All: true, Msg: r.stateEnvelope()},
	}
}

func (r *Room) afterBoard(now time.Time) []Delivery {
	if r.Index >= len(r.Questions)-1 {
		return r.finish()
	}

	return r.advance(now)
}

func (r *Room) advance(now time.Time) []Delivery {
	r.Index++
	if r.Index >= len(r.Questions) {
		return r.finish()
	}

	return r.openQuestion(now)
}

func (r *Room) showBoard(now time.Time) []Delivery {
	r.Phase = PhaseLeaderboard
	r.deadline = now.Add(BoardHold)
	board := Leaderboard{Entries: r.ranked()}

	return []Delivery{
		{Disarm: true},
		{All: true, Msg: r.stateEnvelope()},
		{All: true, Msg: Envelope{Leaderboard: &board}, ArmTimer: true, Deadline: r.deadline},
	}
}

func (r *Room) finish() []Delivery {
	r.Phase = PhaseFinished
	entries := r.ranked()
	results := make([]Result, 0, len(entries))
	for _, entry := range entries {
		results = append(results, Result(entry))
	}

	out := []Delivery{
		{All: true, Msg: r.stateEnvelope()},
		{All: true, Msg: Envelope{GameFinished: &GameFinished{Entries: entries}}},
	}

	if !r.finishedSet {
		r.finishedSet = true
		out[0].Finished = results
	}

	return out
}

func (r *Room) replay(connID string) []Delivery {
	if r.Phase != PhaseQuestion && r.Phase != PhaseReveal && r.Phase != PhaseLeaderboard && r.Phase != PhaseFinished {
		return nil
	}

	out := make([]Delivery, 0, 2)
	if r.Phase == PhaseQuestion || r.Phase == PhaseReveal {
		view := r.questionView(r.Index, r.deadline)
		out = append(out, Delivery{ConnID: connID, Msg: Envelope{QuestionStarted: &view}})
	}

	if r.Phase == PhaseQuestion {
		progress := r.answersProgress()
		out = append(out, Delivery{ConnID: connID, Msg: Envelope{AnswersProgress: &progress}})
	}

	if r.Phase == PhaseReveal {
		revealed := r.Questions[r.Index].reveal()
		out = append(out, Delivery{ConnID: connID, Msg: Envelope{QuestionRevealed: &revealed}})
	}

	if r.Phase == PhaseLeaderboard {
		out = append(out, Delivery{ConnID: connID, Msg: Envelope{Leaderboard: &Leaderboard{Entries: r.ranked()}}})
	}

	if r.Phase == PhaseFinished {
		out = append(out, Delivery{ConnID: connID, Msg: Envelope{GameFinished: &GameFinished{Entries: r.ranked()}}})
	}

	return out
}

func (r *Room) stateEnvelope() Envelope {
	players := make([]Player, 0, len(r.Players))
	for _, item := range r.Players {
		players = append(players, item.view())
	}

	return Envelope{RoomState: &RoomState{Code: r.Code, Phase: string(r.Phase), Players: players}}
}

func (r *Room) playerByConn(connID string) *player {
	for _, item := range r.Players {
		if item.ConnID == connID {
			return item
		}
	}

	return nil
}

func (r *Room) connectedPlayers() []*player {
	out := make([]*player, 0, len(r.Players))
	for _, item := range r.Players {
		if item.Connected {
			out = append(out, item)
		}
	}

	return out
}

func (r *Room) allConnectedAnswered() bool {
	connected := r.connectedPlayers()
	if len(connected) == 0 {
		return false
	}

	for _, item := range connected {
		if _, ok := r.answers[item.ID]; !ok {
			return false
		}
	}

	return true
}

func (r *Room) answersProgress() AnswersProgress {
	connected := r.connectedPlayers()
	ids := make([]string, 0, len(connected))
	for _, item := range connected {
		if _, ok := r.answers[item.ID]; ok {
			ids = append(ids, item.ID)
		}
	}
	slices.Sort(ids)

	return AnswersProgress{
		QuestionID:     r.Questions[r.Index].ID,
		Answered:       len(ids),
		Total:          len(connected),
		AnsweredIDs:    ids,
		DeadlineUnixMs: r.deadline.UnixMilli(),
	}
}

func (r *Room) ranked() []LeaderboardEntry {
	cloned := slices.Clone(r.Players)
	slices.SortFunc(cloned, func(a, b *player) int {
		if a.Score != b.Score {
			return b.Score - a.Score
		}

		return strings.Compare(strings.ToLower(a.Nickname), strings.ToLower(b.Nickname))
	})

	entries := make([]LeaderboardEntry, 0, len(cloned))
	for i, item := range cloned {
		rank := i + 1
		if i > 0 && item.Score == cloned[i-1].Score {
			rank = entries[i-1].Rank
		}

		entries = append(entries, LeaderboardEntry{Nickname: item.Nickname, Score: item.Score, Rank: rank})
	}

	return entries
}

func (p *player) view() Player {
	return Player{ID: p.ID, Nickname: p.Nickname, Score: p.Score, Connected: p.Connected}
}

func (r *Room) questionView(index int, deadline time.Time) QuestionStarted {
	question := r.Questions[index]
	view := question.public(index, len(r.Questions), deadline)
	if question.Category != "" {
		view.Category = question.Category
		view.Round = r.roundOf[index]
		view.RoundTotal = r.roundCount
	}

	return view
}

func (q Question) public(index, total int, deadline time.Time) QuestionStarted {
	options := make([]QuestionOption, 0, len(q.Options))
	for _, option := range q.Options {
		options = append(options, QuestionOption{ID: option.ID, Text: option.Text})
	}

	return QuestionStarted{
		QuestionID:     q.ID,
		Index:          index + 1,
		Total:          total,
		Text:           q.Text,
		Type:           string(q.Type),
		TimeLimitSec:   int(q.Limit / time.Second),
		Options:        options,
		ImageURL:       q.ImageURL,
		DeadlineUnixMs: deadline.UnixMilli(),
	}
}

func (q Question) reveal() QuestionRevealed {
	ids := make([]string, 0, 1)
	for _, option := range q.Options {
		if option.IsCorrect {
			ids = append(ids, option.ID)
		}
	}

	return QuestionRevealed{
		QuestionID:       q.ID,
		CorrectOptionIDs: ids,
		CorrectTexts:     slices.Clone(q.Answers),
	}
}

func (q Question) correct(answer submission) bool {
	switch q.Type {
	case entity.QuestionTypeText:
		got := entity.NormalizeTextAnswer(answer.Text)
		return slices.Contains(q.Answers, got)
	case entity.QuestionTypeSingle:
		if len(answer.OptionIDs) != 1 {
			return false
		}

		for _, option := range q.Options {
			if option.ID == answer.OptionIDs[0] {
				return option.IsCorrect
			}
		}

		return false
	case entity.QuestionTypeMultiple:
		wanted := map[string]struct{}{}
		for _, option := range q.Options {
			if option.IsCorrect {
				wanted[option.ID] = struct{}{}
			}
		}

		if len(answer.OptionIDs) != len(wanted) {
			return false
		}

		seen := map[string]struct{}{}
		for _, id := range answer.OptionIDs {
			if _, ok := wanted[id]; !ok {
				return false
			}

			seen[id] = struct{}{}
		}

		return len(seen) == len(wanted)
	default:
		return false
	}
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

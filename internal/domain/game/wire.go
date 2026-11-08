package game

// Envelope — одно серверное сообщение. Заполнено ровно одно поле.
type Envelope struct {
	RoomState        *RoomState        `json:"room_state,omitempty"`
	PlayerJoined     *PlayerJoined     `json:"player_joined,omitempty"`
	QuestionStarted  *QuestionStarted  `json:"question_started,omitempty"`
	AnswerAccepted   *AnswerAccepted   `json:"answer_accepted,omitempty"`
	QuestionRevealed *QuestionRevealed `json:"question_revealed,omitempty"`
	Leaderboard      *Leaderboard      `json:"leaderboard,omitempty"`
	GameFinished     *GameFinished     `json:"game_finished,omitempty"`
	Error            *GameError        `json:"error,omitempty"`
	PlayerSession    *PlayerSession    `json:"player_session,omitempty"`
	HostResume       *HostResume       `json:"host_resume,omitempty"`
}

type RoomState struct {
	Code    string   `json:"code"`
	Phase   string   `json:"phase"`
	Players []Player `json:"players"`
}

type Player struct {
	ID        string `json:"id"`
	Nickname  string `json:"nickname"`
	Score     int    `json:"score"`
	Connected bool   `json:"connected"`
}

type PlayerJoined struct {
	Player Player `json:"player"`
}

type PlayerSession struct {
	PlayerToken string `json:"player_token"`
	Player      Player `json:"player"`
}

type HostResume struct {
	Token string `json:"token"`
}

type QuestionOption struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type QuestionStarted struct {
	QuestionID     string           `json:"question_id"`
	Index          int              `json:"index"`
	Total          int              `json:"total"`
	Text           string           `json:"text"`
	Type           string           `json:"type"`
	TimeLimitSec   int              `json:"time_limit_sec"`
	Options        []QuestionOption `json:"options,omitempty"`
	DeadlineUnixMs int64            `json:"deadline_unix_ms"`
}

type AnswerAccepted struct {
	QuestionID string `json:"question_id"`
}

type QuestionRevealed struct {
	QuestionID       string   `json:"question_id"`
	CorrectOptionIDs []string `json:"correct_option_ids,omitempty"`
	CorrectTexts     []string `json:"correct_texts,omitempty"`
}

type LeaderboardEntry struct {
	Nickname string `json:"nickname"`
	Score    int    `json:"score"`
	Rank     int    `json:"rank"`
}

type Leaderboard struct {
	Entries []LeaderboardEntry `json:"entries"`
}

type GameFinished struct {
	Entries []LeaderboardEntry `json:"entries"`
}

type GameError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func errorEnvelope(code, message string) Envelope {
	return Envelope{Error: &GameError{Code: code, Message: message}}
}

package adminhttp

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/polarizer700/quizelive-backend/internal/app/usecase"
)

func (s *Server) uploadQuestionImage(w http.ResponseWriter, r *http.Request) {
	quizID, err := uuid.Parse(chi.URLParam(r, "quizId"))
	if err != nil {
		writeUseCaseError(w, r, errInvalidBody())
		return
	}
	questionID, err := uuid.Parse(chi.URLParam(r, "questionId"))
	if err != nil {
		writeUseCaseError(w, r, errInvalidBody())
		return
	}

	// #nosec G120 -- тело запроса уже ограничено (6 МиБ для /image) через MaxBytesReader в limitBody
	if err = r.ParseMultipartForm(6 << 20); err != nil {
		writeUseCaseError(w, r, errInvalidBody())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeUseCaseError(w, r, errInvalidBody())
		return
	}
	defer func() { _ = file.Close() }()

	contentType := header.Header.Get("Content-Type")
	if detected := usecase.ImageContentType(header.Filename); detected != "" {
		contentType = detected
	}

	question, err := s.questions.UploadImage(r.Context(), quizID, questionID, contentType, file, header.Size)
	if err != nil {
		writeUseCaseError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toQuestion(question))
}

func (s *Server) deleteQuestionImage(w http.ResponseWriter, r *http.Request) {
	quizID, err := uuid.Parse(chi.URLParam(r, "quizId"))
	if err != nil {
		writeUseCaseError(w, r, errInvalidBody())
		return
	}
	questionID, err := uuid.Parse(chi.URLParam(r, "questionId"))
	if err != nil {
		writeUseCaseError(w, r, errInvalidBody())
		return
	}

	question, err := s.questions.DeleteImage(r.Context(), quizID, questionID)
	if err != nil {
		writeUseCaseError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toQuestion(question))
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

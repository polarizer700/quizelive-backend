package adminhttp

import (
	"context"

	"github.com/google/uuid"

	"github.com/polarizer700/quizelive-backend/internal/app/usecase"
	"github.com/polarizer700/quizelive-backend/internal/domain/entity"
	"github.com/polarizer700/quizelive-backend/internal/transport/http/openapi"
)

func (s *Server) Login(ctx context.Context, request openapi.LoginRequestObject) (openapi.LoginResponseObject, error) {
	if request.Body == nil {
		return nil, errInvalidBody()
	}

	session, err := s.auth.Login(ctx, request.Body.Email, request.Body.Password)
	if err != nil {
		return nil, err
	}

	s.setRefreshCookie(ctx, session.RefreshToken, session.RefreshExpiresAt)

	return openapi.Login200JSONResponse{
		AccessToken: session.AccessToken,
		ExpiresIn:   int(session.ExpiresIn.Seconds()),
		User:        toUser(session.User),
	}, nil
}

func (s *Server) Refresh(ctx context.Context, _ openapi.RefreshRequestObject) (openapi.RefreshResponseObject, error) {
	session, err := s.auth.Refresh(ctx, refreshFrom(ctx))
	if err != nil {
		return nil, err
	}

	s.setRefreshCookie(ctx, session.RefreshToken, session.RefreshExpiresAt)

	return openapi.Refresh200JSONResponse{
		AccessToken: session.AccessToken,
		ExpiresIn:   int(session.ExpiresIn.Seconds()),
		User:        toUser(session.User),
	}, nil
}

func (s *Server) Logout(ctx context.Context, _ openapi.LogoutRequestObject) (openapi.LogoutResponseObject, error) {
	if err := s.auth.Logout(ctx, refreshFrom(ctx)); err != nil {
		return nil, err
	}

	s.clearRefreshCookie(ctx)

	return openapi.Logout204Response{}, nil
}

func (s *Server) Register(ctx context.Context, request openapi.RegisterRequestObject) (openapi.RegisterResponseObject, error) {
	if request.Body == nil {
		return nil, errInvalidBody()
	}

	if err := s.auth.Register(ctx, string(request.Body.Email), request.Body.Password); err != nil {
		return nil, err
	}

	return openapi.Register204Response{}, nil
}

func (s *Server) VerifyEmail(ctx context.Context, request openapi.VerifyEmailRequestObject) (openapi.VerifyEmailResponseObject, error) {
	if request.Body == nil {
		return nil, errInvalidBody()
	}

	if err := s.auth.VerifyEmail(ctx, request.Body.Token); err != nil {
		return nil, err
	}

	return openapi.VerifyEmail204Response{}, nil
}

func (s *Server) ListUsers(ctx context.Context, request openapi.ListUsersRequestObject) (openapi.ListUsersResponseObject, error) {
	limit, offset := pageArgs(request.Params.Limit, request.Params.Offset)

	users, total, err := s.users.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	return openapi.ListUsers200JSONResponse{Items: toUsers(users), Total: total}, nil
}

func (s *Server) CreateUser(ctx context.Context, request openapi.CreateUserRequestObject) (openapi.CreateUserResponseObject, error) {
	if request.Body == nil {
		return nil, errInvalidBody()
	}

	user, err := s.users.Create(ctx, string(request.Body.Email), request.Body.Password, entity.Role(request.Body.Role))
	if err != nil {
		return nil, err
	}

	return openapi.CreateUser201JSONResponse(toUser(user)), nil
}

func (s *Server) GetUser(ctx context.Context, request openapi.GetUserRequestObject) (openapi.GetUserResponseObject, error) {
	user, err := s.users.Get(ctx, uuidFrom(request.UserId))
	if err != nil {
		return nil, err
	}

	return openapi.GetUser200JSONResponse(toUser(user)), nil
}

func (s *Server) UpdateUser(ctx context.Context, request openapi.UpdateUserRequestObject) (openapi.UpdateUserResponseObject, error) {
	if request.Body == nil {
		return nil, errInvalidBody()
	}

	patch := usecase.UserUpdate{Password: request.Body.Password}
	if request.Body.Email != nil {
		email := string(*request.Body.Email)
		patch.Email = &email
	}

	if request.Body.Role != nil {
		role := entity.Role(*request.Body.Role)
		patch.Role = &role
	}

	user, err := s.users.Update(ctx, uuidFrom(request.UserId), patch)
	if err != nil {
		return nil, err
	}

	return openapi.UpdateUser200JSONResponse(toUser(user)), nil
}

func (s *Server) DeleteUser(ctx context.Context, request openapi.DeleteUserRequestObject) (openapi.DeleteUserResponseObject, error) {
	if err := s.users.Delete(ctx, uuidFrom(request.UserId)); err != nil {
		return nil, err
	}

	return openapi.DeleteUser204Response{}, nil
}

func (s *Server) ListQuizzes(ctx context.Context, request openapi.ListQuizzesRequestObject) (openapi.ListQuizzesResponseObject, error) {
	limit, offset := pageArgs(request.Params.Limit, request.Params.Offset)
	status := ""
	if request.Params.Status != nil {
		status = string(*request.Params.Status)
	}

	quizzes, total, err := s.quizzes.List(ctx, status, limit, offset)
	if err != nil {
		return nil, err
	}

	return openapi.ListQuizzes200JSONResponse{Items: toQuizzes(quizzes), Total: total}, nil
}

func (s *Server) CreateQuiz(ctx context.Context, request openapi.CreateQuizRequestObject) (openapi.CreateQuizResponseObject, error) {
	if request.Body == nil {
		return nil, errInvalidBody()
	}

	description := ""
	if request.Body.Description != nil {
		description = *request.Body.Description
	}

	quiz, err := s.quizzes.Create(ctx, request.Body.Title, description)
	if err != nil {
		return nil, err
	}

	return openapi.CreateQuiz201JSONResponse(toQuiz(quiz)), nil
}

func (s *Server) GetQuiz(ctx context.Context, request openapi.GetQuizRequestObject) (openapi.GetQuizResponseObject, error) {
	quiz, err := s.quizzes.Get(ctx, uuidFrom(request.QuizId))
	if err != nil {
		return nil, err
	}

	return openapi.GetQuiz200JSONResponse(toQuiz(quiz)), nil
}

func (s *Server) UpdateQuiz(ctx context.Context, request openapi.UpdateQuizRequestObject) (openapi.UpdateQuizResponseObject, error) {
	if request.Body == nil {
		return nil, errInvalidBody()
	}

	patch := usecase.QuizUpdate{Title: request.Body.Title, Description: request.Body.Description}
	if request.Body.Status != nil {
		status := entity.QuizStatus(*request.Body.Status)
		patch.Status = &status
	}

	quiz, err := s.quizzes.Update(ctx, uuidFrom(request.QuizId), patch)
	if err != nil {
		return nil, err
	}

	return openapi.UpdateQuiz200JSONResponse(toQuiz(quiz)), nil
}

func (s *Server) DeleteQuiz(ctx context.Context, request openapi.DeleteQuizRequestObject) (openapi.DeleteQuizResponseObject, error) {
	if err := s.quizzes.Delete(ctx, uuidFrom(request.QuizId)); err != nil {
		return nil, err
	}

	return openapi.DeleteQuiz204Response{}, nil
}

func (s *Server) IssueHostToken(ctx context.Context, request openapi.IssueHostTokenRequestObject) (openapi.IssueHostTokenResponseObject, error) {
	token, err := s.quizzes.IssueHostToken(ctx, uuidFrom(request.QuizId))
	if err != nil {
		return nil, err
	}

	return openapi.IssueHostToken200JSONResponse{Token: token.Token, ExpiresIn: token.ExpiresIn}, nil
}

func (s *Server) ListQuestions(ctx context.Context, request openapi.ListQuestionsRequestObject) (openapi.ListQuestionsResponseObject, error) {
	questions, err := s.questions.List(ctx, uuidFrom(request.QuizId))
	if err != nil {
		return nil, err
	}

	return openapi.ListQuestions200JSONResponse(toQuestions(questions)), nil
}

func (s *Server) CreateQuestion(ctx context.Context, request openapi.CreateQuestionRequestObject) (openapi.CreateQuestionResponseObject, error) {
	if request.Body == nil {
		return nil, errInvalidBody()
	}

	question, err := s.questions.Create(ctx, uuidFrom(request.QuizId), fromQuestionInput(*request.Body))
	if err != nil {
		return nil, err
	}

	return openapi.CreateQuestion201JSONResponse(toQuestion(question)), nil
}

func (s *Server) GetQuestion(ctx context.Context, request openapi.GetQuestionRequestObject) (openapi.GetQuestionResponseObject, error) {
	question, err := s.questions.Get(ctx, uuidFrom(request.QuizId), uuidFrom(request.QuestionId))
	if err != nil {
		return nil, err
	}

	return openapi.GetQuestion200JSONResponse(toQuestion(question)), nil
}

func (s *Server) UpdateQuestion(ctx context.Context, request openapi.UpdateQuestionRequestObject) (openapi.UpdateQuestionResponseObject, error) {
	if request.Body == nil {
		return nil, errInvalidBody()
	}

	question, err := s.questions.Update(ctx, uuidFrom(request.QuizId), uuidFrom(request.QuestionId), fromQuestionInput(*request.Body))
	if err != nil {
		return nil, err
	}

	return openapi.UpdateQuestion200JSONResponse(toQuestion(question)), nil
}

func (s *Server) DeleteQuestion(ctx context.Context, request openapi.DeleteQuestionRequestObject) (openapi.DeleteQuestionResponseObject, error) {
	if err := s.questions.Delete(ctx, uuidFrom(request.QuizId), uuidFrom(request.QuestionId)); err != nil {
		return nil, err
	}

	return openapi.DeleteQuestion204Response{}, nil
}

func (s *Server) ReorderQuestions(ctx context.Context, request openapi.ReorderQuestionsRequestObject) (openapi.ReorderQuestionsResponseObject, error) {
	if request.Body == nil {
		return nil, errInvalidBody()
	}

	ids := make([]uuid.UUID, 0, len(request.Body.QuestionIds))
	for _, id := range request.Body.QuestionIds {
		ids = append(ids, uuidFrom(id))
	}

	questions, err := s.questions.Reorder(ctx, uuidFrom(request.QuizId), ids)
	if err != nil {
		return nil, err
	}

	return openapi.ReorderQuestions200JSONResponse(toQuestions(questions)), nil
}

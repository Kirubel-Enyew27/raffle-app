package application

import (
	"context"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
	auditapp "github.com/raffle-app/backend/internal/audit/application"
	"github.com/raffle-app/backend/internal/identity/domain"
	apperrors "github.com/raffle-app/backend/pkg/errors"
	appjwt "github.com/raffle-app/backend/pkg/jwt"
)

type IdentityService struct {
	userRepo     domain.UserRepository
	auditService *auditapp.AuditService
	jwtPrivate   []byte
	jwtExpiry    time.Duration
}

func NewIdentityService(userRepo domain.UserRepository, auditService *auditapp.AuditService, jwtPrivate []byte, jwtExpiry time.Duration) *IdentityService {
	return &IdentityService{
		userRepo:     userRepo,
		auditService: auditService,
		jwtPrivate:   jwtPrivate,
		jwtExpiry:    jwtExpiry,
	}
}

func (s *IdentityService) Register(ctx context.Context, id, email, password string) (*domain.User, error) {
	if email == "" || password == "" {
		return nil, apperrors.ErrValidationFailed
	}

	exists, err := s.userRepo.ExistsByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperrors.ErrConflict
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperrors.ErrInternal
	}

	user := &domain.User{
		ID:           id,
		Email:        email,
		PasswordHash: string(hashed),
		Role:         "user",
		IsBanned:     false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	// Record audit log
	// Registered user is the actor, and since user is register, actorID is user.ID, action is register
	_ = s.auditService.Record(ctx, &user.ID, "user", "register", "user", &user.ID, "", nil, nil)

	return user, nil
}

func (s *IdentityService) Login(ctx context.Context, email, password string) (string, *domain.User, error) {
	if email == "" || password == "" {
		return "", nil, apperrors.ErrValidationFailed
	}

	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return "", nil, err
	}

	if user.IsBanned {
		return "", nil, errors.New("user is banned")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return "", nil, errors.New("invalid credentials")
	}

	token, err := appjwt.GenerateToken(user.ID, user.Email, user.Role, s.jwtPrivate, s.jwtExpiry)
	if err != nil {
		return "", nil, apperrors.ErrInternal
	}

	// Record audit log
	_ = s.auditService.Record(ctx, &user.ID, user.Role, "login", "auth", &user.ID, "", nil, nil)

	return token, user, nil
}

func (s *IdentityService) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	if userID == "" || oldPassword == "" || newPassword == "" {
		return apperrors.ErrValidationFailed
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		// If FindByID is not fully implemented in DB repo, handle it or query from email if needed.
		// Wait, userRepo.FindByID returns nil, nil in current boilerplate! Let's verify that.
		// Let's implement FindByID in postgres.go of identity repo.
		return err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword))
	if err != nil {
		return errors.New("invalid old password")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return apperrors.ErrInternal
	}

	if err := s.userRepo.UpdatePassword(ctx, userID, string(hashed)); err != nil {
		return err
	}

	// Record audit log
	oldVal := "password_masked"
	newVal := "password_changed"
	_ = s.auditService.Record(ctx, &userID, user.Role, "password_change", "user", &userID, "", &oldVal, &newVal)

	return nil
}

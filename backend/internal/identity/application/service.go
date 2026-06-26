package application

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
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
	jwtSecret    []byte
	jwtExpiry    time.Duration
}

func NewIdentityService(userRepo domain.UserRepository, auditService *auditapp.AuditService, jwtSecret []byte, jwtExpiry time.Duration) *IdentityService {
	return &IdentityService{
		userRepo:     userRepo,
		auditService: auditService,
		jwtSecret:    jwtSecret,
		jwtExpiry:    jwtExpiry,
	}
}

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (s *IdentityService) Register(ctx context.Context, id, email, password, fullName, phone string) (*domain.User, error) {
	if (email == "" && phone == "") || password == "" {
		return nil, apperrors.ErrValidationFailed
	}

	if id == "" {
		id = generateUUID()
	}

	if email != "" {
		exists, err := s.userRepo.ExistsByEmail(ctx, email)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, apperrors.ErrConflict
		}
	}

	if phone != "" {
		exists, err := s.userRepo.ExistsByPhone(ctx, phone)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, apperrors.ErrConflict
		}
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperrors.ErrInternal
	}

	user := &domain.User{
		ID:           id,
		Email:        email,
		Phone:        phone,
		FullName:     fullName,
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
	_ = s.auditService.Record(ctx, &user.ID, "user", "register", "user", &user.ID, "", nil, nil)

	return user, nil
}

func (s *IdentityService) Login(ctx context.Context, identifier, password string) (string, *domain.User, error) {
	if identifier == "" || password == "" {
		return "", nil, apperrors.ErrValidationFailed
	}

	// Try email first, then phone
	user, err := s.userRepo.FindByEmail(ctx, identifier)
	if err != nil {
		user, err = s.userRepo.FindByPhone(ctx, identifier)
		if err != nil {
			return "", nil, errors.New("invalid credentials")
		}
	}


	if user.IsBanned {
		return "", nil, errors.New("user is banned")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return "", nil, errors.New("invalid credentials")
	}

	token, err := appjwt.GenerateTokenHMAC(user.ID, user.Email, user.Role, s.jwtSecret, s.jwtExpiry)
	if err != nil {
		return "", nil, apperrors.ErrInternal
	}

	// Record audit log
	_ = s.auditService.Record(ctx, &user.ID, user.Role, "login", "auth", &user.ID, "", nil, nil)

	return token, user, nil
}

func (s *IdentityService) UpdateProfile(ctx context.Context, userID, fullName, phone string, avatarURL ...string) (*domain.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if fullName != "" {
		user.FullName = fullName
	}
	if phone != "" {
		user.Phone = phone
	}
	if len(avatarURL) > 0 && avatarURL[0] != "" {
		user.AvatarURL = avatarURL[0]
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	_ = s.auditService.Record(ctx, &userID, user.Role, "profile_update", "user", &userID, "", nil, nil)
	return user, nil
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

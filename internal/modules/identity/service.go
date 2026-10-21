package identity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/platform/config"
	"github.com/red-copilot/red-x/internal/platform/security"
	"github.com/red-copilot/red-x/internal/store"
)

type Principal struct {
	UserID             string
	Username           string
	Role               domain.Role
	Scopes             []string
	Via                string
	Session            string
	APIKeyID           string
	MustChangePassword bool
}

func (p *Principal) IsAdmin() bool { return p != nil && p.Role == domain.RoleAdmin }
func (p *Principal) CanWrite() bool {
	return p != nil && (p.Role == domain.RoleAdmin || p.Role == domain.RoleOperator)
}

type Service struct {
	store *store.Store
	cfg   config.Config
	log   *slog.Logger
}

func New(st *store.Store, cfg config.Config, log *slog.Logger) *Service {
	return &Service{store: st, cfg: cfg, log: log}
}

func (s *Service) BootstrapAdmin(ctx context.Context) error {
	n, err := s.store.CountUsers(ctx)
	if err != nil || n > 0 {
		return err
	}
	password := s.cfg.Auth.BootstrapPassword
	generated := false
	if password == "" {
		password, err = security.RandomToken(18)
		if err != nil {
			return err
		}
		generated = true
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return err
	}
	user := &domain.User{
		Username: "admin", PasswordHash: hash, Role: domain.RoleAdmin,
		MustChangePassword: true,
	}
	if err := s.store.CreateUser(ctx, user); err != nil {
		return err
	}
	if generated {
		s.log.Warn("bootstrap admin created; change this password immediately", "username", "admin", "password", password)
	} else {
		s.log.Info("bootstrap admin created", "username", "admin")
	}
	return nil
}

type LoginResult struct {
	User           domain.User
	AccessToken    string
	RefreshToken   string
	AccessExpires  time.Time
	RefreshExpires time.Time
	CSRFToken      string
	SessionID      string
}

func (s *Service) Login(ctx context.Context, username, password, ip, ua string) (*LoginResult, error) {
	u, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if u == nil || u.Disabled || !security.VerifyPassword(password, u.PasswordHash) {
		return nil, errors.New("invalid username or password")
	}
	accessTTL, refreshTTL := s.cfg.AccessTTL(), s.cfg.RefreshTTL()
	refresh, err := security.RandomToken(32)
	if err != nil {
		return nil, err
	}
	csrf, err := security.RandomToken(24)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	sess := &domain.Session{
		UserID: u.ID, RefreshHash: security.SHA256Hex(refresh), UserAgent: ua, IP: ip,
		ExpiresAt: now.Add(refreshTTL),
	}
	if err := s.store.CreateSession(ctx, sess); err != nil {
		return nil, err
	}
	access, err := security.NewAccessToken(s.cfg.Auth.AccessSecret, u.ID, u.Username, string(u.Role), sess.ID, accessTTL)
	if err != nil {
		return nil, err
	}
	s.store.AddAudit(ctx, domain.AuditEvent{ActorID: u.ID, Actor: u.Username, Action: "auth.login", Resource: "session", IP: ip})
	return &LoginResult{
		User: *u, AccessToken: access, RefreshToken: refresh,
		AccessExpires: now.Add(accessTTL), RefreshExpires: now.Add(refreshTTL),
		CSRFToken: csrf, SessionID: sess.ID,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken, ip, ua string) (*LoginResult, error) {
	if refreshToken == "" {
		return nil, errors.New("missing refresh token")
	}
	accessTTL, refreshTTL := s.cfg.AccessTTL(), s.cfg.RefreshTTL()
	newRefresh, err := security.RandomToken(32)
	if err != nil {
		return nil, err
	}
	csrf, err := security.RandomToken(24)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	sess, err := s.store.RotateSession(ctx, security.SHA256Hex(refreshToken), security.SHA256Hex(newRefresh), now.Add(refreshTTL))
	if err != nil {
		return nil, err
	}
	u, err := s.store.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return nil, err
	}
	if u == nil || u.Disabled {
		_ = s.store.RevokeSession(ctx, sess.ID)
		return nil, errors.New("user disabled")
	}
	access, err := security.NewAccessToken(s.cfg.Auth.AccessSecret, u.ID, u.Username, string(u.Role), sess.ID, accessTTL)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		User: *u, AccessToken: access, RefreshToken: newRefresh,
		AccessExpires: now.Add(accessTTL), RefreshExpires: now.Add(refreshTTL),
		CSRFToken: csrf, SessionID: sess.ID,
	}, nil
}

func (s *Service) Logout(ctx context.Context, p *Principal) {
	if p != nil && p.Session != "" {
		_ = s.store.RevokeSession(ctx, p.Session)
	}
}

func (s *Service) Authenticate(ctx context.Context, accessToken, apiKey string) (*Principal, error) {
	if apiKey != "" {
		hash := security.APIKeyHash(apiKey, s.cfg.Auth.APIKeyPepper)
		key, err := s.store.GetAPIKeyByHash(ctx, hash)
		if err != nil {
			return nil, err
		}
		if key == nil {
			return nil, errors.New("invalid api key")
		}
		s.store.TouchAPIKey(ctx, key.ID)
		role := domain.RoleViewer
		if security.HasScope(key.Scopes, "admin") {
			role = domain.RoleAdmin
		} else if security.HasScope(key.Scopes, "scan") {
			role = domain.RoleOperator
		}
		return &Principal{Username: "api-key:" + key.Name, Role: role, Scopes: key.Scopes, Via: "api_key", APIKeyID: key.ID}, nil
	}
	if accessToken == "" {
		return nil, errors.New("missing access token")
	}
	claims, err := security.ParseAccessToken(s.cfg.Auth.AccessSecret, accessToken)
	if err != nil {
		return nil, err
	}
	userID, err := domain.ParseID(claims.UserID)
	if err != nil {
		return nil, err
	}
	u, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u == nil || u.Disabled {
		return nil, errors.New("user disabled")
	}
	sessionID := ""
	if claims.SessionID != "" {
		sessionID, _ = domain.ParseID(claims.SessionID)
	}
	if sessionID != "" {
		active, err := s.store.SessionActive(ctx, sessionID)
		if err != nil {
			return nil, err
		}
		if !active {
			return nil, errors.New("session revoked or expired")
		}
	}
	return &Principal{
		UserID: u.ID, Username: u.Username, Role: u.Role, Via: "cookie",
		Session: sessionID, MustChangePassword: u.MustChangePassword,
	}, nil
}

func (s *Service) ChangePassword(ctx context.Context, p *Principal, oldPassword, newPassword string) error {
	if p == nil || p.UserID == "" {
		return errors.New("only interactive users can change passwords")
	}
	u, err := s.store.GetUserByID(ctx, p.UserID)
	if err != nil || u == nil {
		return errors.New("user not found")
	}
	if !security.VerifyPassword(oldPassword, u.PasswordHash) {
		return errors.New("old password is incorrect")
	}
	hash, err := security.HashPassword(newPassword)
	if err != nil {
		return err
	}
	u.PasswordHash = hash
	u.MustChangePassword = false
	if err := s.store.UpdateUser(ctx, u); err != nil {
		return err
	}
	_ = s.store.RevokeUserSessions(ctx, u.ID)
	s.store.AddAudit(ctx, domain.AuditEvent{ActorID: u.ID, Actor: u.Username, Action: "auth.change_password", Resource: "user"})
	return nil
}

func (s *Service) CreateUser(ctx context.Context, username, password string, role domain.Role) (*domain.User, error) {
	username = strings.TrimSpace(username)
	if username == "" || len(password) < 10 {
		return nil, errors.New("username is required and password must be at least 10 characters")
	}
	switch role {
	case domain.RoleViewer, domain.RoleOperator, domain.RoleAdmin:
	default:
		return nil, errors.New("invalid role")
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return nil, err
	}
	u := &domain.User{Username: username, PasswordHash: hash, Role: role, MustChangePassword: true}
	if err := s.store.CreateUser(ctx, u); err != nil {
		if strings.Contains(err.Error(), "E11000") {
			return nil, errors.New("username already exists")
		}
		return nil, err
	}
	return u, nil
}

func (s *Service) ResetPassword(ctx context.Context, id string, password string) error {
	if len(password) < 10 {
		return errors.New("password must be at least 10 characters")
	}
	u, err := s.store.GetUserByID(ctx, id)
	if err != nil || u == nil {
		return errors.New("user not found")
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return err
	}
	u.PasswordHash = hash
	u.MustChangePassword = true
	if err := s.store.UpdateUser(ctx, u); err != nil {
		return err
	}
	return s.store.RevokeUserSessions(ctx, id)
}

func (s *Service) CreateAPIKey(ctx context.Context, p *Principal, name string, scopes []string) (string, domain.APIKey, error) {
	if p == nil || !p.IsAdmin() {
		return "", domain.APIKey{}, errors.New("admin role required")
	}
	raw, err := security.RandomToken(32)
	if err != nil {
		return "", domain.APIKey{}, err
	}
	for i, sc := range scopes {
		if sc != "read" && sc != "scan" && sc != "admin" {
			return "", domain.APIKey{}, fmt.Errorf("invalid scope %q", scopes[i])
		}
	}
	key := &domain.APIKey{
		Name: name, KeyHash: security.APIKeyHash(raw, s.cfg.Auth.APIKeyPepper),
		Scopes: scopes, CreatedBy: p.UserID,
	}
	if err := s.store.CreateAPIKey(ctx, key); err != nil {
		return "", domain.APIKey{}, err
	}
	return raw, *key, nil
}

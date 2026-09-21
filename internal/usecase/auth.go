package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/carolineeey/wedding-suite-api/internal/models"
	"golang.org/x/crypto/bcrypt"
)

const (
	// sessionTTL is fixed, not sliding: a couple logs in a handful of times
	// before the wedding, and a month is long enough not to be a nuisance.
	sessionTTL = 30 * 24 * time.Hour

	minPasswordChars = 10
	// maxPasswordBytes is bcrypt's input limit; longer passwords would be
	// rejected by bcrypt itself rather than silently truncated.
	maxPasswordBytes = 72
	maxEmailChars    = 254
)

// ErrInvalidCredentials is a failed login or an unknown, expired, or revoked
// session token. It deliberately does not say which, so a caller cannot probe
// for registered emails.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrNoAdminInScope guards admin-owned writes against running without an
// authenticated admin. The auth middleware sets one before any admin route
// runs, so reaching this means a route was mounted outside it.
var ErrNoAdminInScope = errors.New("no admin in scope")

// AdminStore is the admin account storage AuthUsecase needs.
type AdminStore interface {
	// Create inserts an admin and returns its ID, or models.ErrDuplicate if
	// the email is taken.
	Create(ctx context.Context, email, passwordHash string) (string, error)
	// ByEmail returns the admin and its password hash.
	ByEmail(ctx context.Context, email string) (models.Admin, string, error)
	SetPassword(ctx context.Context, adminID, passwordHash string) error
}

// WeddingAccessStore records which admins may manage which weddings.
type WeddingAccessStore interface {
	GrantAdmin(ctx context.Context, weddingID, adminID string) error
	HasAdmin(ctx context.Context, weddingID, adminID string) (bool, error)
	ListByAdmin(ctx context.Context, adminID string) ([]models.Wedding, error)
}

// SessionStore keeps login sessions, keyed by the hash of their token.
type SessionStore interface {
	Create(ctx context.Context, tokenHash, adminID string, expiresAt time.Time) error
	// AdminByToken returns the admin whose session has this token hash and
	// has not expired at now, or models.ErrNotFound.
	AdminByToken(ctx context.Context, tokenHash string, now time.Time) (models.Admin, error)
	Delete(ctx context.Context, tokenHash string) error
	DeleteForAdmin(ctx context.Context, adminID string) error
	DeleteExpired(ctx context.Context, adminID string, now time.Time) error
}

// AuthUsecase handles admin accounts, login sessions, and which weddings an
// admin may manage.
type AuthUsecase struct {
	admins   AdminStore
	access   WeddingAccessStore
	sessions SessionStore
	scope    *WeddingScope

	now           func() time.Time
	newToken      func() (string, error)
	hashPassword  func(password string) (string, error)
	checkPassword func(hash, password string) error
}

// dummyHash is checked against when the email is unknown, so a login for a
// missing account takes as long as one with a wrong password. It is computed
// on first use rather than at startup; a fixed input well under the bcrypt
// limit cannot fail to hash.
var dummyHash = sync.OnceValue(func() string {
	hash, _ := bcryptHash("dummy-password-for-timing")
	return hash
})

func NewAuthUsecase(admins AdminStore, access WeddingAccessStore, sessions SessionStore, scope *WeddingScope) *AuthUsecase {
	return &AuthUsecase{
		admins:        admins,
		access:        access,
		sessions:      sessions,
		scope:         scope,
		now:           time.Now,
		newToken:      generateSessionToken,
		hashPassword:  bcryptHash,
		checkPassword: bcryptCheck,
	}
}

// Session is a successful login: the bearer token to send on admin
// requests, when it stops working, and who it belongs to.
type Session struct {
	Token     string
	ExpiresAt time.Time
	Admin     models.Admin
}

// Login checks an admin's email and password and starts a new session.
// A wrong password and an unknown email are both ErrInvalidCredentials.
func (u *AuthUsecase) Login(ctx context.Context, email, password string) (Session, error) {
	email = normalizeEmail(email)
	if email == "" || password == "" {
		return Session{}, invalid("email and password are required")
	}

	admin, hash, err := u.admins.ByEmail(ctx, email)
	if errors.Is(err, models.ErrNotFound) {
		_ = u.checkPassword(dummyHash(), password)
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, fmt.Errorf("loading admin: %w", err)
	}
	if u.checkPassword(hash, password) != nil {
		return Session{}, ErrInvalidCredentials
	}

	now := u.now()
	// Pruning on login keeps the table from growing without a cron job.
	if err := u.sessions.DeleteExpired(ctx, admin.ID, now); err != nil {
		return Session{}, fmt.Errorf("pruning sessions: %w", err)
	}
	token, err := u.newToken()
	if err != nil {
		return Session{}, fmt.Errorf("generating session token: %w", err)
	}
	expiresAt := now.Add(sessionTTL)
	if err := u.sessions.Create(ctx, hashToken(token), admin.ID, expiresAt); err != nil {
		return Session{}, fmt.Errorf("saving session: %w", err)
	}
	return Session{Token: token, ExpiresAt: expiresAt, Admin: admin}, nil
}

// Authenticate returns the admin a bearer token belongs to, or
// ErrInvalidCredentials if the token is unknown, expired, or logged out.
func (u *AuthUsecase) Authenticate(ctx context.Context, token string) (models.Admin, error) {
	if token == "" {
		return models.Admin{}, ErrInvalidCredentials
	}
	admin, err := u.sessions.AdminByToken(ctx, hashToken(token), u.now())
	if errors.Is(err, models.ErrNotFound) {
		return models.Admin{}, ErrInvalidCredentials
	}
	if err != nil {
		return models.Admin{}, fmt.Errorf("loading session: %w", err)
	}
	return admin, nil
}

// Logout ends the session the token belongs to. Logging out twice is not an
// error: either way the token no longer works.
func (u *AuthUsecase) Logout(ctx context.Context, token string) error {
	if err := u.sessions.Delete(ctx, hashToken(token)); err != nil && !errors.Is(err, models.ErrNotFound) {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

// CanManage reports whether the admin may manage the wedding the slug names.
// A wedding the admin was not granted is models.ErrNotFound, the same as an
// unknown slug, so admins cannot discover other couples' slugs.
func (u *AuthUsecase) CanManage(ctx context.Context, adminID, slug string) error {
	weddingID, err := u.scope.BySlug(ctx, slug)
	if err != nil {
		return err
	}
	ok, err := u.access.HasAdmin(ctx, weddingID, adminID)
	if err != nil {
		return fmt.Errorf("checking wedding access: %w", err)
	}
	if !ok {
		return models.ErrNotFound
	}
	return nil
}

// Weddings lists the weddings the admin may manage.
func (u *AuthUsecase) Weddings(ctx context.Context, adminID string) ([]models.Wedding, error) {
	if adminID == "" {
		return nil, ErrNoAdminInScope
	}
	return u.access.ListByAdmin(ctx, adminID)
}

// CreateAdmin registers a new admin account. There is no signup endpoint;
// this runs from the cmd/admin tool.
func (u *AuthUsecase) CreateAdmin(ctx context.Context, email, password string) (models.Admin, error) {
	email = normalizeEmail(email)
	if err := validateEmail(email); err != nil {
		return models.Admin{}, err
	}
	hash, err := u.validHash(password)
	if err != nil {
		return models.Admin{}, err
	}
	if _, err := u.admins.Create(ctx, email, hash); err != nil {
		if errors.Is(err, models.ErrDuplicate) {
			return models.Admin{}, invalid("email is already registered")
		}
		return models.Admin{}, fmt.Errorf("creating admin: %w", err)
	}
	admin, _, err := u.admins.ByEmail(ctx, email)
	return admin, err
}

// SetPassword replaces an admin's password and ends all of their sessions,
// so a password reset also locks out anyone holding an old token.
func (u *AuthUsecase) SetPassword(ctx context.Context, email, password string) error {
	hash, err := u.validHash(password)
	if err != nil {
		return err
	}
	admin, _, err := u.admins.ByEmail(ctx, normalizeEmail(email))
	if err != nil {
		return err
	}
	if err := u.admins.SetPassword(ctx, admin.ID, hash); err != nil {
		return fmt.Errorf("saving password: %w", err)
	}
	if err := u.sessions.DeleteForAdmin(ctx, admin.ID); err != nil {
		return fmt.Errorf("revoking sessions: %w", err)
	}
	return nil
}

// Grant lets the admin with this email manage the wedding the slug names.
// Granting twice is a no-op.
func (u *AuthUsecase) Grant(ctx context.Context, email, slug string) error {
	admin, _, err := u.admins.ByEmail(ctx, normalizeEmail(email))
	if err != nil {
		return fmt.Errorf("admin %q: %w", email, err)
	}
	weddingID, err := u.scope.BySlug(ctx, slug)
	if err != nil {
		return fmt.Errorf("wedding %q: %w", slug, err)
	}
	return u.access.GrantAdmin(ctx, weddingID, admin.ID)
}

func (u *AuthUsecase) validHash(password string) (string, error) {
	if utf8.RuneCountInString(password) < minPasswordChars {
		return "", invalid("password must be at least %d characters", minPasswordChars)
	}
	if len(password) > maxPasswordBytes {
		return "", invalid("password must be at most %d bytes", maxPasswordBytes)
	}
	hash, err := u.hashPassword(password)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	return hash, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// validateEmail only checks the shape: there is no email delivery to prove
// the address is real, and the operator creating the account typed it.
func validateEmail(email string) error {
	local, domain, ok := strings.Cut(email, "@")
	if !ok || local == "" || domain == "" || strings.ContainsAny(email, " \t\r\n") || strings.Contains(domain, "@") {
		return invalid("email must be a valid email address")
	}
	if tooLong(email, maxEmailChars) {
		return invalid("email must be at most %d characters", maxEmailChars)
	}
	return nil
}

// generateSessionToken returns 32 random bytes, URL-safe encoded, so the
// token is unguessable and fits in an Authorization header as is.
func generateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken is what the session table stores. The token already carries 256
// bits of randomness, so a fast unsalted hash is enough; bcrypt would only
// slow down every admin request.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func bcryptHash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func bcryptCheck(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

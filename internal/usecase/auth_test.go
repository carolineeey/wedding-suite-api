package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/carolineeey/wedding-suite-api/internal/models"
)

var authNow = time.Date(2027, 1, 10, 12, 0, 0, 0, time.UTC)

const testPassword = "correct horse battery"

type authFixture struct {
	uc       *AuthUsecase
	admins   *fakeAdmins
	weddings *fakeWeddings
	sessions *fakeSessions
}

// newAuthFixture swaps bcrypt for a readable fake hash so tests stay fast
// and can assert the password was hashed rather than stored raw.
func newAuthFixture() authFixture {
	admins := &fakeAdmins{}
	weddings := &fakeWeddings{wedding: testWedding()}
	sessions := &fakeSessions{admins: admins}
	uc := NewAuthUsecase(admins, weddings, sessions, NewWeddingScope(weddings))
	uc.now = func() time.Time { return authNow }
	uc.hashPassword = func(pw string) (string, error) { return "hashed:" + pw, nil }
	uc.checkPassword = func(hash, pw string) error {
		if hash != "hashed:"+pw {
			return errors.New("mismatch")
		}
		return nil
	}
	tokens := 0
	uc.newToken = func() (string, error) {
		tokens++
		return "token-" + string(rune('0'+tokens)), nil
	}
	return authFixture{uc, admins, weddings, sessions}
}

func (fx authFixture) createAdmin(t *testing.T, email string) models.Admin {
	t.Helper()
	admin, err := fx.uc.CreateAdmin(context.Background(), email, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	return admin
}

func TestCreateAdmin(t *testing.T) {
	fx := newAuthFixture()

	admin := fx.createAdmin(t, "  Caroline@Example.com ")
	if admin.Email != "caroline@example.com" || admin.ID == "" {
		t.Errorf("admin = %+v, want a trimmed, lowercased email and an ID", admin)
	}
	if got := fx.admins.byEmail["caroline@example.com"].hash; got != "hashed:"+testPassword {
		t.Errorf("stored hash = %q, want the hashed password", got)
	}
}

func TestCreateAdminValidation(t *testing.T) {
	tests := []struct {
		name, email, password string
	}{
		{"missing email", "", testPassword},
		{"no at sign", "caroline.example.com", testPassword},
		{"no domain", "caroline@", testPassword},
		{"two at signs", "a@b@c", testPassword},
		{"space in email", "caro line@example.com", testPassword},
		{"email too long", strings.Repeat("a", maxEmailChars) + "@example.com", testPassword},
		{"password too short", "a@example.com", "short"},
		{"password over bcrypt limit", "a@example.com", strings.Repeat("a", maxPasswordBytes+1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fx := newAuthFixture()
			_, err := fx.uc.CreateAdmin(context.Background(), tt.email, tt.password)
			var verr *ValidationError
			if !errors.As(err, &verr) {
				t.Errorf("err = %v, want ValidationError", err)
			}
			if len(fx.admins.byEmail) != 0 {
				t.Error("admin stored despite invalid input")
			}
		})
	}
}

func TestCreateAdminDuplicateEmail(t *testing.T) {
	fx := newAuthFixture()
	fx.createAdmin(t, "a@example.com")

	_, err := fx.uc.CreateAdmin(context.Background(), "A@example.com", testPassword)
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Errorf("err = %v, want ValidationError", err)
	}
}

func TestLogin(t *testing.T) {
	fx := newAuthFixture()
	admin := fx.createAdmin(t, "a@example.com")

	session, err := fx.uc.Login(context.Background(), " A@Example.com", testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if session.Admin.ID != admin.ID || session.Token == "" {
		t.Errorf("session = %+v, want a token for %s", session, admin.ID)
	}
	if want := authNow.Add(sessionTTL); !session.ExpiresAt.Equal(want) {
		t.Errorf("expires_at = %v, want %v", session.ExpiresAt, want)
	}
	if _, stored := fx.sessions.byHash[session.Token]; stored {
		t.Error("raw token stored; only its hash should be")
	}
	if _, stored := fx.sessions.byHash[hashToken(session.Token)]; !stored {
		t.Error("session not stored under the token hash")
	}
}

// A wrong password and an unknown email must look identical to the caller,
// so login cannot be used to find out who has an account.
func TestLoginRejects(t *testing.T) {
	tests := []struct {
		name, email, password string
		wantErr               error
	}{
		{"wrong password", "a@example.com", "not the password", ErrInvalidCredentials},
		{"unknown email", "nobody@example.com", testPassword, ErrInvalidCredentials},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fx := newAuthFixture()
			fx.createAdmin(t, "a@example.com")

			_, err := fx.uc.Login(context.Background(), tt.email, tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if len(fx.sessions.byHash) != 0 {
				t.Error("session created for a failed login")
			}
		})
	}
}

func TestLoginRequiresFields(t *testing.T) {
	fx := newAuthFixture()
	_, err := fx.uc.Login(context.Background(), "  ", "")
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Errorf("err = %v, want ValidationError", err)
	}
}

func TestLoginPrunesExpiredSessions(t *testing.T) {
	fx := newAuthFixture()
	admin := fx.createAdmin(t, "a@example.com")
	_ = fx.sessions.Create(context.Background(), "old", admin.ID, authNow.Add(-time.Hour))

	if _, err := fx.uc.Login(context.Background(), "a@example.com", testPassword); err != nil {
		t.Fatal(err)
	}
	if _, ok := fx.sessions.byHash["old"]; ok {
		t.Error("expired session kept after login")
	}
}

func TestAuthenticate(t *testing.T) {
	fx := newAuthFixture()
	admin := fx.createAdmin(t, "a@example.com")
	session, err := fx.uc.Login(context.Background(), "a@example.com", testPassword)
	if err != nil {
		t.Fatal(err)
	}

	got, err := fx.uc.Authenticate(context.Background(), session.Token)
	if err != nil || got.ID != admin.ID {
		t.Errorf("Authenticate = %+v, %v; want %s", got, err, admin.ID)
	}

	for name, token := range map[string]string{"empty": "", "unknown": "forged"} {
		if _, err := fx.uc.Authenticate(context.Background(), token); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("%s token: err = %v, want ErrInvalidCredentials", name, err)
		}
	}

	fx.uc.now = func() time.Time { return session.ExpiresAt }
	if _, err := fx.uc.Authenticate(context.Background(), session.Token); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expired token: err = %v, want ErrInvalidCredentials", err)
	}
}

func TestLogout(t *testing.T) {
	fx := newAuthFixture()
	fx.createAdmin(t, "a@example.com")
	session, err := fx.uc.Login(context.Background(), "a@example.com", testPassword)
	if err != nil {
		t.Fatal(err)
	}

	if err := fx.uc.Logout(context.Background(), session.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.uc.Authenticate(context.Background(), session.Token); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("token still works after logout: err = %v", err)
	}
	// Logging out an already-ended session is not an error.
	if err := fx.uc.Logout(context.Background(), session.Token); err != nil {
		t.Errorf("second logout: err = %v, want nil", err)
	}
}

func TestSetPasswordRevokesSessions(t *testing.T) {
	fx := newAuthFixture()
	fx.createAdmin(t, "a@example.com")
	session, err := fx.uc.Login(context.Background(), "a@example.com", testPassword)
	if err != nil {
		t.Fatal(err)
	}

	const newPassword = "a brand new password"
	if err := fx.uc.SetPassword(context.Background(), "A@example.com", newPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.uc.Authenticate(context.Background(), session.Token); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("old session still works after a password change: err = %v", err)
	}
	if _, err := fx.uc.Login(context.Background(), "a@example.com", testPassword); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("old password: err = %v, want ErrInvalidCredentials", err)
	}
	if _, err := fx.uc.Login(context.Background(), "a@example.com", newPassword); err != nil {
		t.Errorf("new password: err = %v", err)
	}
}

func TestSetPasswordValidation(t *testing.T) {
	fx := newAuthFixture()
	fx.createAdmin(t, "a@example.com")

	err := fx.uc.SetPassword(context.Background(), "a@example.com", "short")
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Errorf("err = %v, want ValidationError", err)
	}
}

// An admin must only reach weddings they were granted. A wedding they were
// not granted is reported exactly like one that does not exist.
func TestCanManage(t *testing.T) {
	fx := newAuthFixture()
	owner := fx.createAdmin(t, "owner@example.com")
	other := fx.createAdmin(t, "other@example.com")
	if err := fx.uc.Grant(context.Background(), "owner@example.com", "s"); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name, adminID, slug string
		wantErr             error
	}{
		{"granted", owner.ID, "s", nil},
		{"granted, slug in other case", owner.ID, " S ", nil},
		{"not granted", other.ID, "s", models.ErrNotFound},
		{"unknown slug", owner.ID, "ghost", models.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fx.uc.CanManage(context.Background(), tt.adminID, tt.slug)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestGrant(t *testing.T) {
	ctx := context.Background()

	t.Run("unknown admin", func(t *testing.T) {
		fx := newAuthFixture()
		if err := fx.uc.Grant(ctx, "nobody@example.com", "s"); !errors.Is(err, models.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
	t.Run("unknown wedding", func(t *testing.T) {
		fx := newAuthFixture()
		fx.createAdmin(t, "a@example.com")
		if err := fx.uc.Grant(ctx, "a@example.com", "ghost"); !errors.Is(err, models.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
		if len(fx.weddings.grants) != 0 {
			t.Error("grant recorded for an unknown wedding")
		}
	})
	t.Run("lists granted weddings", func(t *testing.T) {
		fx := newAuthFixture()
		admin := fx.createAdmin(t, "a@example.com")
		if err := fx.uc.Grant(ctx, "a@example.com", "s"); err != nil {
			t.Fatal(err)
		}
		weddings, err := fx.uc.Weddings(ctx, admin.ID)
		if err != nil || len(weddings) != 1 || weddings[0].ID != "wedding-1" {
			t.Errorf("Weddings = %+v, %v; want wedding-1", weddings, err)
		}
	})
}

package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/carolineeey/wedding-suite-api/internal/models"
)

// In-memory stores for usecase tests. They record what they were asked to
// store so tests can assert on it.

var (
	_ WeddingStore = (*fakeWeddings)(nil)
	_ EventLister  = (*fakeEvents)(nil)
	_ EventStore   = (*fakeEvents)(nil)
	_ GuestStore   = (*fakeGuests)(nil)
	_ RSVPStore    = (*fakeRSVPStore)(nil)
	_ WishStore    = (*fakeWishes)(nil)
	_ GiftStore    = (*fakeGifts)(nil)
	_ WeddingByID  = (*fakeWeddings)(nil)

	_ AdminStore         = (*fakeAdmins)(nil)
	_ WeddingAccessStore = (*fakeWeddings)(nil)
	_ SessionStore       = (*fakeSessions)(nil)
)

func ptr[T any](v T) *T { return &v }

// scopedID records the wedding a scoped write was aimed at, so tests can
// assert the wedding scope actually reaches the store.
type scopedID struct{ weddingID, id string }

func testWedding() *models.Wedding {
	return &models.Wedding{ID: "wedding-1", Slug: "s", PartnerOneName: "A", PartnerTwoName: "B"}
}

type fakeWeddings struct {
	wedding          *models.Wedding
	creates, updates int
	loseCreateRace   bool       // Create reports that another request took the slug first
	takenSlug        string     // a slug some other wedding already holds
	owner            string     // admin ID the last Create granted
	grants           []scopedID // (weddingID, adminID) pairs GrantAdmin recorded
}

func (f *fakeWeddings) SlugTaken(_ context.Context, slug, excludeID string) (bool, error) {
	if f.takenSlug != "" && slug == f.takenSlug {
		return true, nil
	}
	if f.wedding == nil || f.wedding.ID == excludeID {
		return false, nil
	}
	return f.wedding.Slug == slug, nil
}

func (f *fakeWeddings) BySlug(_ context.Context, slug string) (models.Wedding, error) {
	if f.wedding == nil || f.wedding.Slug != slug {
		return models.Wedding{}, models.ErrNotFound
	}
	return *f.wedding, nil
}

func (f *fakeWeddings) ByID(_ context.Context, id string) (models.Wedding, error) {
	if f.wedding == nil || f.wedding.ID != id {
		return models.Wedding{}, models.ErrNotFound
	}
	return *f.wedding, nil
}

func (f *fakeWeddings) Create(_ context.Context, w models.Wedding, ownerID string) error {
	f.creates++
	if f.loseCreateRace {
		// Stand in for another request taking the slug first.
		f.wedding = testWedding()
		return models.ErrDuplicate
	}
	w.ID = "wedding-1"
	f.wedding = &w
	f.owner = ownerID
	return nil
}

func (f *fakeWeddings) Update(_ context.Context, w models.Wedding) error {
	if f.wedding == nil || f.wedding.ID != w.ID {
		return models.ErrNotFound
	}
	f.updates++
	f.wedding = &w
	return nil
}

func (f *fakeWeddings) GrantAdmin(_ context.Context, weddingID, adminID string) error {
	f.grants = append(f.grants, scopedID{weddingID, adminID})
	return nil
}

func (f *fakeWeddings) HasAdmin(_ context.Context, weddingID, adminID string) (bool, error) {
	for _, g := range f.grants {
		if g == (scopedID{weddingID, adminID}) {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeWeddings) ListByAdmin(ctx context.Context, adminID string) ([]models.Wedding, error) {
	out := []models.Wedding{}
	if f.wedding != nil {
		if ok, _ := f.HasAdmin(ctx, f.wedding.ID, adminID); ok {
			out = append(out, *f.wedding)
		}
	}
	return out, nil
}

type fakeAdmin struct {
	admin models.Admin
	hash  string
}

type fakeAdmins struct {
	byEmail map[string]*fakeAdmin
}

func (f *fakeAdmins) Create(_ context.Context, email, passwordHash string) (string, error) {
	if f.byEmail == nil {
		f.byEmail = map[string]*fakeAdmin{}
	}
	if _, ok := f.byEmail[email]; ok {
		return "", models.ErrDuplicate
	}
	id := fmt.Sprintf("admin-%d", len(f.byEmail)+1)
	f.byEmail[email] = &fakeAdmin{models.Admin{ID: id, Email: email}, passwordHash}
	return id, nil
}

func (f *fakeAdmins) ByEmail(_ context.Context, email string) (models.Admin, string, error) {
	a, ok := f.byEmail[email]
	if !ok {
		return models.Admin{}, "", models.ErrNotFound
	}
	return a.admin, a.hash, nil
}

func (f *fakeAdmins) SetPassword(_ context.Context, adminID, passwordHash string) error {
	for _, a := range f.byEmail {
		if a.admin.ID == adminID {
			a.hash = passwordHash
			return nil
		}
	}
	return models.ErrNotFound
}

type fakeSession struct {
	admin     models.Admin
	expiresAt time.Time
}

// fakeSessions is keyed by token hash, like the real table. admins resolves
// admin IDs back to admins for AdminByToken.
type fakeSessions struct {
	admins *fakeAdmins
	byHash map[string]fakeSession
}

func (f *fakeSessions) Create(_ context.Context, tokenHash, adminID string, expiresAt time.Time) error {
	if f.byHash == nil {
		f.byHash = map[string]fakeSession{}
	}
	var admin models.Admin
	for _, a := range f.admins.byEmail {
		if a.admin.ID == adminID {
			admin = a.admin
		}
	}
	f.byHash[tokenHash] = fakeSession{admin, expiresAt}
	return nil
}

func (f *fakeSessions) AdminByToken(_ context.Context, tokenHash string, now time.Time) (models.Admin, error) {
	s, ok := f.byHash[tokenHash]
	if !ok || !s.expiresAt.After(now) {
		return models.Admin{}, models.ErrNotFound
	}
	return s.admin, nil
}

func (f *fakeSessions) Delete(_ context.Context, tokenHash string) error {
	if _, ok := f.byHash[tokenHash]; !ok {
		return models.ErrNotFound
	}
	delete(f.byHash, tokenHash)
	return nil
}

func (f *fakeSessions) DeleteForAdmin(_ context.Context, adminID string) error {
	for h, s := range f.byHash {
		if s.admin.ID == adminID {
			delete(f.byHash, h)
		}
	}
	return nil
}

func (f *fakeSessions) DeleteExpired(_ context.Context, adminID string, now time.Time) error {
	for h, s := range f.byHash {
		if s.admin.ID == adminID && !s.expiresAt.After(now) {
			delete(f.byHash, h)
		}
	}
	return nil
}

type fakeEvents struct {
	created []models.Event
	deleted []scopedID
}

func (f *fakeEvents) ListByWedding(context.Context, string) ([]models.Event, error) {
	return f.created, nil
}

func (f *fakeEvents) Create(_ context.Context, e models.Event) (string, error) {
	f.created = append(f.created, e)
	return fmt.Sprintf("event-%d", len(f.created)), nil
}

func (f *fakeEvents) Delete(_ context.Context, weddingID, id string) error {
	f.deleted = append(f.deleted, scopedID{weddingID, id})
	return nil
}

type fakeGuests struct {
	createErrs  []error // returned by successive Create calls before succeeding
	createCalls int
	created     []models.Guest
	updated     []models.Guest
	deleted     []scopedID
}

func (f *fakeGuests) ListByWedding(context.Context, string) ([]models.Guest, error) {
	return f.created, nil
}

func (f *fakeGuests) Create(_ context.Context, g models.Guest) (string, error) {
	f.createCalls++
	if len(f.createErrs) > 0 {
		err := f.createErrs[0]
		f.createErrs = f.createErrs[1:]
		if err != nil {
			return "", err
		}
	}
	f.created = append(f.created, g)
	return fmt.Sprintf("guest-%d", len(f.created)), nil
}

func (f *fakeGuests) Update(_ context.Context, g models.Guest) error {
	f.updated = append(f.updated, g)
	return nil
}

func (f *fakeGuests) Delete(_ context.Context, weddingID, id string) error {
	f.deleted = append(f.deleted, scopedID{weddingID, id})
	return nil
}

func (f *fakeGuests) Summary(context.Context, string) (models.RSVPSummary, error) {
	return models.RSVPSummary{}, nil
}

type savedRSVP struct {
	id             string
	status         models.RSVPStatus
	attendingCount int
	message        string
	respondedAt    time.Time
}

type fakeRSVPStore struct {
	byCode map[string]models.Guest
	saved  *savedRSVP
}

func (f *fakeRSVPStore) GetByInviteCode(_ context.Context, code string) (models.Guest, error) {
	g, ok := f.byCode[code]
	if !ok {
		return models.Guest{}, models.ErrNotFound
	}
	return g, nil
}

func (f *fakeRSVPStore) SaveRSVP(_ context.Context, id string, status models.RSVPStatus, attendingCount int, message string, respondedAt time.Time) error {
	f.saved = &savedRSVP{id, status, attendingCount, message, respondedAt}
	return nil
}

type fakeWishes struct {
	created     []models.Wish
	listedLimit int
	approved    []scopedID
	deleted     []scopedID
}

func (f *fakeWishes) List(_ context.Context, _ string, _ bool, limit int) ([]models.Wish, error) {
	f.listedLimit = limit
	return f.created, nil
}

func (f *fakeWishes) Create(_ context.Context, w models.Wish) (string, error) {
	f.created = append(f.created, w)
	return fmt.Sprintf("wish-%d", len(f.created)), nil
}

func (f *fakeWishes) SetApproval(_ context.Context, weddingID, id string, _ bool) error {
	f.approved = append(f.approved, scopedID{weddingID, id})
	return nil
}

func (f *fakeWishes) Delete(_ context.Context, weddingID, id string) error {
	f.deleted = append(f.deleted, scopedID{weddingID, id})
	return nil
}

type fakeGifts struct {
	created []models.GiftAccount
	deleted []scopedID
}

func (f *fakeGifts) ListByWedding(context.Context, string) ([]models.GiftAccount, error) {
	return f.created, nil
}

func (f *fakeGifts) Create(_ context.Context, g models.GiftAccount) (string, error) {
	f.created = append(f.created, g)
	return fmt.Sprintf("gift-%d", len(f.created)), nil
}

func (f *fakeGifts) Delete(_ context.Context, weddingID, id string) error {
	f.deleted = append(f.deleted, scopedID{weddingID, id})
	return nil
}

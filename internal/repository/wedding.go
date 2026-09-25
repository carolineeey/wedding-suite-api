package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/carolineeey/wedding-suite-api/internal/errtrace"
	"github.com/carolineeey/wedding-suite-api/internal/models"
)

type WeddingRepository struct {
	db *sql.DB
}

func NewWeddingRepository(db *sql.DB) *WeddingRepository {
	return &WeddingRepository{db: db}
}

// weddingColumns is shared by every query that loads whole weddings.
// wedding_date is cast to text: lib/pq decodes DATE as a time.Time, which
// would serialize as "2027-06-12T00:00:00Z" and render as the previous day in
// browsers west of UTC.
const weddingColumns = `w.id, w.slug, w.partner_one_name, w.partner_two_name, w.wedding_date::text,
	COALESCE(w.opening_text, ''), COALESCE(w.story, ''), COALESCE(w.dress_code, ''), w.created_at`

func scanWedding(row rowScanner) (models.Wedding, error) {
	var w models.Wedding
	var weddingDate sql.NullString
	if err := row.Scan(&w.ID, &w.Slug, &w.PartnerOneName, &w.PartnerTwoName, &weddingDate,
		&w.OpeningText, &w.Story, &w.DressCode, &w.CreatedAt); err != nil {
		return w, err
	}
	if weddingDate.Valid {
		w.WeddingDate = &weddingDate.String
	}
	return w, nil
}

// BySlug fetches the wedding a URL names. The slug is unique, so this is the
// lookup every request funnels through.
func (r *WeddingRepository) BySlug(ctx context.Context, slug string) (models.Wedding, error) {
	w, err := scanWedding(r.db.QueryRowContext(ctx, `
		SELECT `+weddingColumns+`
		FROM weddings w
		WHERE w.slug = $1
	`, slug))
	if errors.Is(err, sql.ErrNoRows) {
		return w, models.ErrNotFound
	}
	return w, errtrace.Wrap(err)
}

// ByID fetches a wedding by its ID. The invitation page uses it: a guest's
// invite code leads to their wedding by ID, not by slug.
func (r *WeddingRepository) ByID(ctx context.Context, id string) (models.Wedding, error) {
	w, err := scanWedding(r.db.QueryRowContext(ctx, `
		SELECT `+weddingColumns+`
		FROM weddings w
		WHERE w.id = $1
	`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return w, models.ErrNotFound
	}
	return w, errtrace.Wrap(err)
}

// SlugTaken reports whether a wedding other than excludeID already uses this
// slug. excludeID is compared as text so that "" (a wedding being created)
// matches no row instead of failing the uuid cast.
func (r *WeddingRepository) SlugTaken(ctx context.Context, slug, excludeID string) (bool, error) {
	var taken bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM weddings WHERE slug = $1 AND id::text <> $2)
	`, slug, excludeID).Scan(&taken)
	return taken, errtrace.Wrap(err)
}

// Create inserts the wedding and grants ownerID access to it in one
// transaction, so a failed grant never leaves a wedding nobody can manage.
// It returns models.ErrDuplicate if the slug is taken.
func (r *WeddingRepository) Create(ctx context.Context, w models.Wedding, ownerID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return errtrace.Wrap(err)
	}
	defer tx.Rollback()

	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO weddings (slug, partner_one_name, partner_two_name, wedding_date,
		                      opening_text, story, dress_code)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''))
		RETURNING id
	`, w.Slug, w.PartnerOneName, w.PartnerTwoName, w.WeddingDate,
		w.OpeningText, w.Story, w.DressCode).Scan(&id)
	if isUniqueViolation(err) {
		return models.ErrDuplicate
	}
	if err != nil {
		return errtrace.Wrap(err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO wedding_admins (wedding_id, admin_id) VALUES ($1, $2)
	`, id, ownerID); err != nil {
		return errtrace.Wrap(err)
	}
	return errtrace.Wrap(tx.Commit())
}

// Update saves the wedding. Renaming it onto a slug another wedding holds is
// models.ErrDuplicate, not a raw driver error.
func (r *WeddingRepository) Update(ctx context.Context, w models.Wedding) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE weddings
		SET slug = $1, partner_one_name = $2, partner_two_name = $3, wedding_date = $4,
		    opening_text = NULLIF($5, ''), story = NULLIF($6, ''), dress_code = NULLIF($7, '')
		WHERE id = $8
	`, w.Slug, w.PartnerOneName, w.PartnerTwoName, w.WeddingDate,
		w.OpeningText, w.Story, w.DressCode, w.ID)
	if isUniqueViolation(err) {
		return models.ErrDuplicate
	}
	return requireAffected(res, err)
}

// GrantAdmin lets the admin manage the wedding. Granting twice is a no-op.
func (r *WeddingRepository) GrantAdmin(ctx context.Context, weddingID, adminID string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO wedding_admins (wedding_id, admin_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, weddingID, adminID)
	return errtrace.Wrap(err)
}

// HasAdmin reports whether the admin may manage the wedding.
func (r *WeddingRepository) HasAdmin(ctx context.Context, weddingID, adminID string) (bool, error) {
	var ok bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM wedding_admins WHERE wedding_id = $1 AND admin_id = $2)
	`, weddingID, adminID).Scan(&ok)
	return ok, errtrace.Wrap(err)
}

// ListByAdmin returns the weddings the admin may manage, soonest first.
func (r *WeddingRepository) ListByAdmin(ctx context.Context, adminID string) ([]models.Wedding, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+weddingColumns+`
		FROM weddings w
		JOIN wedding_admins wa ON wa.wedding_id = w.id
		WHERE wa.admin_id = $1
		ORDER BY w.wedding_date NULLS LAST, w.created_at
	`, adminID)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	defer rows.Close()

	weddings := []models.Wedding{}
	for rows.Next() {
		w, err := scanWedding(rows)
		if err != nil {
			return nil, errtrace.Wrap(err)
		}
		weddings = append(weddings, w)
	}
	return weddings, errtrace.Wrap(rows.Err())
}

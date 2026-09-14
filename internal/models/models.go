package models

import "time"

// Wedding is the top-level tenant. There's only one row today, but every
// other table is scoped by WeddingID so a second wedding can be added later
// without a schema change.
type Wedding struct {
	ID             string    `json:"id"`
	Slug           string    `json:"slug"`
	PartnerOneName string    `json:"partner_one_name"`
	PartnerTwoName string    `json:"partner_two_name"`
	WeddingDate    *string   `json:"wedding_date,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// Event is a single item on the wedding day schedule (ceremony, reception, etc).
type Event struct {
	ID        string     `json:"id"`
	WeddingID string     `json:"wedding_id"`
	Name      string     `json:"name"`
	StartsAt  time.Time  `json:"starts_at"`
	EndsAt    *time.Time `json:"ends_at,omitempty"`
	VenueName string     `json:"venue_name,omitempty"`
	Address   string     `json:"address,omitempty"`
	Notes     string     `json:"notes,omitempty"`
	SortOrder int        `json:"sort_order"`
}

// RSVPStatus enumerates the allowed guest RSVP states.
type RSVPStatus string

const (
	RSVPPending   RSVPStatus = "pending"
	RSVPAttending RSVPStatus = "attending"
	RSVPDeclined  RSVPStatus = "declined"
)

// Guest is one invitee (or invited household/group) tied to a unique invite code.
type Guest struct {
	ID              string     `json:"id"`
	WeddingID       string     `json:"wedding_id"`
	InviteCode      string     `json:"invite_code"`
	Name            string     `json:"name"`
	GroupName       string     `json:"group_name,omitempty"`
	MaxGuests       int        `json:"max_guests"`
	RSVPStatus      RSVPStatus `json:"rsvp_status"`
	AttendingCount  int        `json:"attending_count"`
	RSVPMessage     string     `json:"rsvp_message,omitempty"`
	RSVPRespondedAt *time.Time `json:"rsvp_responded_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// Wish is a guestbook-style message left by a guest.
type Wish struct {
	ID         string    `json:"id"`
	WeddingID  string    `json:"wedding_id"`
	GuestName  string    `json:"guest_name"`
	Message    string    `json:"message"`
	IsApproved bool      `json:"is_approved"`
	CreatedAt  time.Time `json:"created_at"`
}

// RSVPSummary is the aggregate count used on the admin dashboard.
type RSVPSummary struct {
	TotalGuestRecords  int `json:"total_guest_records"`
	TotalInvitedPeople int `json:"total_invited_people"`
	Pending            int `json:"pending"`
	Attending          int `json:"attending"`
	AttendingPeople    int `json:"attending_people"`
	Declined           int `json:"declined"`
}

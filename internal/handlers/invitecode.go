package handlers

import (
	"crypto/rand"
	"strings"
)

// alphabet excludes visually ambiguous characters (0/O, 1/I/L) since invite
// codes get typed by hand or read off a printed card.
const inviteCodeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

func generateInviteCode(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.Grow(length)
	for _, v := range b {
		sb.WriteByte(inviteCodeAlphabet[int(v)%len(inviteCodeAlphabet)])
	}
	return sb.String(), nil
}

package usecase

import (
	"crypto/rand"
	"strings"
)

const inviteCodeLength = 7

// inviteCodeAlphabet excludes visually ambiguous characters (0/O, 1/I/L)
// since invite codes get typed by hand or read off a printed card.
const inviteCodeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

func generateInviteCode() (string, error) {
	b := make([]byte, inviteCodeLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.Grow(inviteCodeLength)
	for _, v := range b {
		sb.WriteByte(inviteCodeAlphabet[int(v)%len(inviteCodeAlphabet)])
	}
	return sb.String(), nil
}

// normalizeInviteCode lets guests type their code in any case and with
// stray whitespace; generated codes are always uppercase.
func normalizeInviteCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

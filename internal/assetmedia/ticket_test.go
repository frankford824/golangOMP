package assetmedia

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"
)

func TestTicketRejectsWrongAudienceTamperingExpiryAndPurpose(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Unix(1700000000, 0)
	claim := Ticket{Version: 1, KeyID: "v1", GatewayID: "office", ActorID: 42, ResourceKind: "task_asset", ResourceID: "12", SourceVersion: "ta:12", ContentID: strings.Repeat("a", 64), Purpose: "download", Rendition: "original", IssuedAt: now.Unix(), ExpiresAt: now.Add(10 * time.Minute).Unix()}
	token, err := SignTicket(claim, priv)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]ed25519.PublicKey{"v1": pub}
	if _, err = VerifyTicket(token, "office", keys, now); err != nil {
		t.Fatal(err)
	}
	if _, err = VerifyTicket(token, "other", keys, now); err == nil {
		t.Fatal("accepted wrong audience")
	}
	if _, err = VerifyTicket(token, "office", keys, now.Add(10*time.Minute)); err == nil {
		t.Fatal("accepted expired ticket")
	}
	p := strings.Split(token, ".")
	p[0] = "A" + p[0][1:]
	if _, err = VerifyTicket(strings.Join(p, "."), "office", keys, now); err == nil {
		t.Fatal("accepted changed payload")
	}
	claim.Purpose = "preview"
	if _, err = SignTicket(claim, priv); err == nil {
		t.Fatal("accepted original as preview")
	}
}

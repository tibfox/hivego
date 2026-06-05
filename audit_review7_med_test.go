package hivego

// review7 hivego MED fixes (pure-MED; CRIT/HIGH-dual-labeled ones excluded).

import (
	"bytes"
	"testing"
	"time"
)

// HG-M8: appendVAsset must reject a negative amount instead of serializing it
// as a two's-complement int64 (which reads back as a huge positive on Hive).
func TestAuditReview7_HGM8_RejectsNegativeAmount(t *testing.T) {
	var neg bytes.Buffer
	if err := appendVAsset("-5.000 HIVE", &neg); err == nil {
		t.Error("HG-M8: negative amount must be rejected")
	}
	var ok bytes.Buffer
	if err := appendVAsset("5.000 HIVE", &ok); err != nil {
		t.Errorf("HG-M8: a valid amount must still serialize: %v", err)
	}
}

// HG-M9: GphBase58Encode must include the version byte so the encode/decode
// pair round-trips.
func TestAuditReview7_HGM9_Base58RoundTrip(t *testing.T) {
	input := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	version := [1]byte{0x80}
	payload, gotVersion, err := GphBase58CheckDecode(GphBase58Encode(input, version))
	if err != nil {
		t.Fatalf("HG-M9: round-trip decode failed: %v", err)
	}
	if gotVersion != version {
		t.Errorf("HG-M9: version mismatch: got %v want %v", gotVersion, version)
	}
	if !bytes.Equal(payload, input) {
		t.Errorf("HG-M9: payload mismatch: got %v want %v", payload, input)
	}
}

// HG-M10: opIdB must fail loudly on an unregistered op instead of returning 0
// (vote_operation) and silently mis-serializing it. Resolved as a returned
// error (not a panic) so SerializeOp callers can fail closed.
func TestAuditReview7_HGM10_UnknownOpFailsLoud(t *testing.T) {
	if id, err := opIdB("vote"); err != nil || id != 0 {
		t.Errorf("HG-M10: vote must remain op id 0, got id=%d err=%v", id, err)
	}
	if id, err := opIdB("custom_json"); err != nil || id == 0 {
		t.Errorf("HG-M10: a known non-vote op must not collide with vote (0), got id=%d err=%v", id, err)
	}
	if _, err := opIdB("definitely_not_a_real_op"); err == nil {
		t.Error("HG-M10: an unregistered op must error, not map to vote (0)")
	}
}

// HG-M10 (follow-up): a directly-constructed ClaimRewardOperation has an empty
// opText; now that OpName returns the registered literal, it must serialize
// cleanly (op id 39) instead of failing/panicking as an "unknown operation".
func TestAuditReview7_HGM10_ClaimRewardDirectConstruct(t *testing.T) {
	op := ClaimRewardOperation{
		Account:     "alice",
		RewardHBD:   "0.000 HBD",
		RewardHIVE:  "0.000 HIVE",
		RewardVests: "0.000000 VESTS",
	}
	b, err := op.SerializeOp()
	if err != nil {
		t.Fatalf("HG-M10: directly-constructed ClaimRewardOperation must serialize, got err: %v", err)
	}
	if len(b) == 0 || b[0] != 39 {
		t.Fatalf("HG-M10: claim_reward_balance op id must be 39, got first byte %v", b)
	}
}

// HG-M11: a short/empty block_id must not panic the BlockID[0:8] slice.
// Resolved against main's review2 #21 fix: blockNumFromID returns an explicit
// error on malformed input rather than silently yielding 0.
func TestAuditReview7_HGM11_ShortBlockID(t *testing.T) {
	if _, err := blockNumFromID(""); err == nil {
		t.Error("HG-M11: empty id must error, not panic")
	}
	if _, err := blockNumFromID("abc"); err == nil {
		t.Error("HG-M11: short id must error, not panic")
	}
	if got, err := blockNumFromID("0000000a1234567890"); err != nil || got != 10 {
		t.Errorf("HG-M11: '0000000a...' -> %d, err %v, want 10, nil", got, err)
	}
}

// HG-M14: appendVStringArray must use a varint length prefix (graphene vector),
// not a single byte that wraps at 256 / is invalid at >= 128.
func TestAuditReview7_HGM14_ArrayVarintLength(t *testing.T) {
	// < 128: varint is the identical single byte.
	var small bytes.Buffer
	appendVStringArray([]string{"a", "b"}, &small)
	if small.Bytes()[0] != 2 {
		t.Errorf("HG-M14: small array prefix = %d, want 2", small.Bytes()[0])
	}
	// 200 entries: varint(200) = 0xC8 0x01 (two bytes), not a single 0xC8.
	big := make([]string, 200)
	for i := range big {
		big[i] = "x"
	}
	var b bytes.Buffer
	appendVStringArray(big, &b)
	out := b.Bytes()
	if out[0] != 0xC8 || out[1] != 0x01 {
		t.Errorf("HG-M14: prefix for 200 entries = %x %x, want c8 01 (varint)", out[0], out[1])
	}
}

// HG-M16: ValidateExpiration rejects an expired/malformed tx; accepts a future one.
func TestAuditReview7_HGM16_ValidateExpiration(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	if (&HiveTransaction{Expiration: "2016-08-08T12:24:17"}).ValidateExpiration(now) == nil {
		t.Error("HG-M16: an expired tx must be rejected")
	}
	if (&HiveTransaction{Expiration: "not-a-date"}).ValidateExpiration(now) == nil {
		t.Error("HG-M16: a malformed expiration must be rejected")
	}
	if err := (&HiveTransaction{Expiration: "2030-01-01T00:00:00"}).ValidateExpiration(now); err != nil {
		t.Errorf("HG-M16: a future tx must validate: %v", err)
	}
}

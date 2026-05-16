package hivego

import "testing"

// review2 HIGH #20 — DecodePublicKey sliced pubKey[:len(PublicKeyPrefix)] and
// decoded[len-4:] with no length checks, panicking (slice bounds out of
// range) on short/garbage input instead of returning an error.
func TestDecodePublicKey_ShortInputReturnsErrorNotPanic(t *testing.T) {
	for _, in := range []string{"", "S", "ST", "STM", "STM1", "X", "STMshort"} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("DecodePublicKey(%q) panicked: %v", in, r)
				}
			}()
			pk, err := DecodePublicKey(in)
			if err == nil {
				t.Fatalf("DecodePublicKey(%q) = %v, want error", in, pk)
			}
		}()
	}
}

// review2 HIGH #79 — FetchVirtualOps built the RPC params with
// OnlyVirtual: IncludeReversible, so the caller's onlyVirtual argument was
// ignored and ALL ops were fetched instead of virtual-only.
func TestNewGetVirtualOpsParams_MapsOnlyVirtualFromArg(t *testing.T) {
	p := newGetVirtualOpsParams(123, true, false)
	if p.BlockNum != 123 {
		t.Fatalf("BlockNum = %d, want 123", p.BlockNum)
	}
	if !p.OnlyVirtual {
		t.Fatalf("OnlyVirtual = false, want true (must follow onlyVirtual arg, not include_reversible)")
	}
	if p.IncludeReversible {
		t.Fatalf("IncludeReversible = true, want false")
	}
	p2 := newGetVirtualOpsParams(9, false, true)
	if p2.OnlyVirtual || !p2.IncludeReversible {
		t.Fatalf("got OnlyVirtual=%v IncludeReversible=%v, want false/true", p2.OnlyVirtual, p2.IncludeReversible)
	}
}

// review2 HIGH #21 — block.BlockID[0:8] panicked on a short/empty block_id
// (slice bounds out of range) and the hex error was discarded.
func TestBlockNumFromID(t *testing.T) {
	// 40-char hex block id: first 4 bytes (8 hex) = big-endian block number.
	id := "02b8c2f9" + "00000000000000000000000000000000000000000000000000000000000000"[:32]
	n, err := blockNumFromID(id)
	if err != nil {
		t.Fatalf("valid id: unexpected error %v", err)
	}
	if n != 0x02b8c2f9 {
		t.Fatalf("blockNumFromID = %d, want %d", n, 0x02b8c2f9)
	}
	for _, bad := range []string{"", "ab", "0123456", "nothexyz"} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("blockNumFromID(%q) panicked: %v", bad, r)
				}
			}()
			if _, err := blockNumFromID(bad); err == nil {
				t.Fatalf("blockNumFromID(%q) = nil error, want error", bad)
			}
		}()
	}
}

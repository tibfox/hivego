package hivego

import "testing"

// review2 #46 — TransferOperation / TransferToSavings / TransferFromSavings
// SerializeOp discarded appendVAsset's error. A malformed Amount (not
// "<num> <SYM>") therefore serialized a transfer with NO asset bytes and
// returned a nil error, so callers signed/broadcast a corrupt op.
//
// Differential: #170 baseline returns (bytes, nil) for a garbage amount
// (RED); fix returns a non-nil error (GREEN). A valid amount still
// serializes with no error on both arms (sanity).
func TestReview2TransferSerializeRejectsBadAsset(t *testing.T) {
	bad := "not-a-valid-asset"
	good := "1.000 HIVE"

	cases := []struct {
		name string
		ser  func(amount string) ([]byte, error)
	}{
		{"transfer", func(a string) ([]byte, error) {
			return TransferOperation{From: "alice", To: "bob", Amount: a, Memo: ""}.SerializeOp()
		}},
		{"transfer_to_savings", func(a string) ([]byte, error) {
			return TransferToSavings{From: "alice", To: "bob", Amount: a, Memo: ""}.SerializeOp()
		}},
		{"transfer_from_savings", func(a string) ([]byte, error) {
			return TransferFromSavings{From: "alice", To: "bob", Amount: a, Memo: "", RequestId: 1}.SerializeOp()
		}},
	}

	for _, c := range cases {
		if _, err := c.ser(bad); err == nil {
			t.Fatalf("review2 #46 (%s): bad amount %q serialized with nil error "+
				"(baseline discards appendVAsset err and emits a corrupt op)", c.name, bad)
		}
		if _, err := c.ser(good); err != nil {
			t.Fatalf("review2 #46 (%s): valid amount %q must still serialize, got err: %v", c.name, good, err)
		}
	}
}

// review2 #102 — serializeAuthority logged-and-silently-returned on write
// errors and did a bare accountAuth[1].(int) / keyAuth[1].(int) assertion,
// so a non-int weight PANICKED and any write failure produced a truncated
// authority that callers signed as valid. Exercised via the public
// AccountCreateOperation.SerializeOp (same signature on both arms).
//
// Differential: #170 baseline PANICS on a non-int weight (RED); fix
// returns a non-nil error (GREEN). A well-formed authority still
// serializes cleanly on both arms (sanity).
func TestReview2SerializeAuthorityBadTupleNoPanic(t *testing.T) {
	badWeight := AccountCreateOperation{
		Fee:            "1.000 HIVE",
		Creator:        "alice",
		NewAccountName: "bob",
		Owner: Auths{
			WeightThreshold: 1,
			// weight is a string, not an int — baseline panics here.
			AccountAuths: [][2]interface{}{{"alice", "1"}},
		},
		MemoKey: "STM8GC13uCZbP44HzMLV6zPZGwVQ8Nt4Kji8PapsPiNq1BK153XTX",
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("review2 #102: AccountCreateOperation.SerializeOp panicked on a "+
					"non-int auth weight: %v (baseline bare .(int) assertion)", r)
			}
		}()
		if _, err := badWeight.SerializeOp(); err == nil {
			t.Fatalf("review2 #102: malformed authority serialized with nil error " +
				"(must fail closed so callers don't sign a truncated authority)")
		}
	}()

	// review2 #102 (sortKeyAuth gap): a non-string KEY in key_auths with >=2
	// entries panicked in sortKeyAuth's bare .(string) comparator, BEFORE
	// serializeAuthority's comma-ok guard could catch it. Must now error, not
	// panic.
	badKey := AccountCreateOperation{
		Fee:            "1.000 HIVE",
		Creator:        "alice",
		NewAccountName: "bob",
		Owner: Auths{
			WeightThreshold: 1,
			// first key is an int, not a string — sortKeyAuth panicked here.
			KeyAuths: [][2]interface{}{{12345, 1}, {"STMkey", 1}},
		},
		MemoKey: "STM8GC13uCZbP44HzMLV6zPZGwVQ8Nt4Kji8PapsPiNq1BK153XTX",
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("review2 #102: SerializeOp panicked on a non-string key_auths "+
					"key via sortKeyAuth: %v (must fail closed with an error instead)", r)
			}
		}()
		if _, err := badKey.SerializeOp(); err == nil {
			t.Fatalf("review2 #102: non-string key_auths key serialized with nil error " +
				"(must fail closed so callers don't sign a truncated authority)")
		}
	}()

	// Sanity: a well-formed authority serializes without error on both arms.
	okOp := AccountCreateOperation{
		Fee:            "1.000 HIVE",
		Creator:        "alice",
		NewAccountName: "bob",
		Owner:          Auths{WeightThreshold: 1, AccountAuths: [][2]interface{}{{"alice", 1}}},
		Active:         Auths{WeightThreshold: 1, AccountAuths: [][2]interface{}{{"alice", 1}}},
		Posting:        Auths{WeightThreshold: 1, AccountAuths: [][2]interface{}{{"alice", 1}}},
		MemoKey:        "STM8GC13uCZbP44HzMLV6zPZGwVQ8Nt4Kji8PapsPiNq1BK153XTX",
	}
	if _, err := okOp.SerializeOp(); err != nil {
		t.Fatalf("review2 #102: well-formed authority must still serialize, got err: %v", err)
	}
}

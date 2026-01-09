package hivego

import (
	"testing"
)

func TestFailoverWithMultipleNodes(t *testing.T) {
	// Test with one good node and one bad node
	// Using a real endpoint and an invalid one to simulate failure
	addrs := []string{"https://api.hive.blog", "https://invalid.endpoint.com"}
	rpc := NewHiveRpc(addrs)

	// This should succeed on the first node
	_, err := rpc.GetDynamicGlobalProps()
	if err != nil {
		t.Errorf("Expected success with failover, but got error: %v", err)
	}

	// Check that currentIndex is set to the successful node (index 0)
	if rpc.currentIndex != 0 {
		t.Errorf("Expected currentIndex to be 0 (last successful), got %d", rpc.currentIndex)
	}

	// Check stats: first node should have success count > 0
	if rpc.nodeStats[0].successCount == 0 {
		t.Errorf("Expected success count > 0 for first node, got %d", rpc.nodeStats[0].successCount)
	}
}

func TestFailoverAllNodesFail(t *testing.T) {
	// Test with all invalid nodes
	addrs := []string{"https://invalid1.com", "https://invalid2.com"}
	rpc := NewHiveRpc(addrs)

	_, err := rpc.GetDynamicGlobalProps()
	if err == nil {
		t.Errorf("Expected failure when all nodes fail, but got success")
	}

	// Check that error message indicates all failed
	expectedMsg := "all API nodes failed"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestRoundRobinPriority(t *testing.T) {
	// Test that it starts from currentIndex and updates to last successful
	addrs := []string{"https://invalid.com", "https://api.hive.blog"}
	rpc := NewHiveRpc(addrs)

	// First call should fail on invalid, succeed on second, set currentIndex to 1
	_, err := rpc.GetDynamicGlobalProps()
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	if rpc.currentIndex != 1 {
		t.Errorf("Expected currentIndex to be 1, got %d", rpc.currentIndex)
	}

	// Second call should start from index 1 (the last successful)
	rpc.currentIndex = 0 // Reset to test
	_, err = rpc.GetDynamicGlobalProps()
	if err != nil {
		t.Errorf("Expected success on second call, got error: %v", err)
	}

	// Should have tried invalid first (failure), then api.hive.blog (success)
	if rpc.nodeStats[0].failureCount == 0 {
		t.Errorf("Expected failure count > 0 for invalid node")
	}
	if rpc.nodeStats[1].successCount == 0 {
		t.Errorf("Expected success count > 0 for valid node")
	}
}

func TestRollingAverage(t *testing.T) {
	addrs := []string{"https://api.hive.blog"}
	rpc := NewHiveRpc(addrs)

	// Make a successful call
	_, err := rpc.GetDynamicGlobalProps()
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	// Rolling average should be 1.0 (1 success, 0 failures)
	expectedAvg := 1.0
	if rpc.nodeStats[0].rollingAvg != expectedAvg {
		t.Errorf("Expected rolling average %f, got %f", expectedAvg, rpc.nodeStats[0].rollingAvg)
	}
}

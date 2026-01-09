package hivego

import (
	"errors"
	"log"
	"sync"

	"github.com/cfoxon/jsonrpc2client"
)

type NodeStats struct {
	successCount int
	failureCount int
	rollingAvg   float64
}

type HiveRpcNode struct {
	addresses    []string
	currentIndex int
	nodeStats    []NodeStats
	mutex        sync.RWMutex
	MaxConn      int
	MaxBatch     int
	NoBroadcast  bool
}

type globalProps struct {
	HeadBlockNumber int    `json:"head_block_number"`
	HeadBlockId     string `json:"head_block_id"`
	Time            string `json:"time"`
}

type hrpcQuery struct {
	method string
	params interface{}
}

func NewHiveRpc(addrs []string) *HiveRpcNode {
	return NewHiveRpcWithOpts(addrs, 1, 1)
}

func NewHiveRpcWithOpts(addrs []string, maxConn int, maxBatch int) *HiveRpcNode {
	nodeStats := make([]NodeStats, len(addrs))
	return &HiveRpcNode{
		addresses:    addrs,
		currentIndex: 0,
		nodeStats:    nodeStats,
		MaxConn:      maxConn,
		MaxBatch:     maxBatch,
	}
}

func (h *HiveRpcNode) GetDynamicGlobalProps() ([]byte, error) {
	q := hrpcQuery{method: "condenser_api.get_dynamic_global_properties", params: []string{}}
	res, err := h.rpcExec(q)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (h *HiveRpcNode) rpcExec(query hrpcQuery) ([]byte, error) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	numNodes := len(h.addresses)
	for i := 0; i < numNodes; i++ {
		index := (h.currentIndex + i) % numNodes
		endpoint := h.addresses[index]

		rpcClient := jsonrpc2client.NewClientWithOpts(endpoint, h.MaxConn, h.MaxBatch)
		jr2query := &jsonrpc2client.RpcRequest{Method: query.method, JsonRpc: "2.0", Id: 1, Params: query.params}
		resp, err := rpcClient.CallRaw(jr2query)
		if err != nil {
			if enableLogging {
				log.Printf("rpcExec failed for endpoint %s (index %d), method %s: %v", endpoint, index, query.method, err)
			}
			h.nodeStats[index].failureCount++
			h.updateRollingAvg(index)
			if enableLogging {
				h.logFailureCounts()
				nextIndex := (h.currentIndex + i + 1) % numNodes
				h.logSwitchingNode(index, nextIndex, numNodes)
			}
			continue
		}

		if resp.Error != nil {
			if enableLogging {
				log.Printf("rpcExec received error response from endpoint %s (index %d), method %s: %v", endpoint, index, query.method, resp.Error)
			}
			h.nodeStats[index].failureCount++
			h.updateRollingAvg(index)
			if enableLogging {
				h.logFailureCounts()
				nextIndex := (h.currentIndex + i + 1) % numNodes
				h.logSwitchingNode(index, nextIndex, numNodes)
			}
			continue
		}

		// Check for bad data: if result is empty, consider it bad
		if len(resp.Result) == 0 {
			if enableLogging {
				log.Printf("rpcExec received empty result from endpoint %s (index %d), method %s", endpoint, index, query.method)
			}
			h.nodeStats[index].failureCount++
			h.updateRollingAvg(index)
			if enableLogging {
				h.logFailureCounts()
				nextIndex := (h.currentIndex + i + 1) % numNodes
				h.logSwitchingNode(index, nextIndex, numNodes)
			}
			continue
		}

		// Success
		h.nodeStats[index].successCount++
		h.updateRollingAvg(index)
		h.currentIndex = index // Set to last successful node
		return resp.Result, nil
	}

	return nil, errors.New("all API nodes failed")
}

func (h *HiveRpcNode) updateRollingAvg(index int) {
	total := h.nodeStats[index].successCount + h.nodeStats[index].failureCount
	if total > 0 {
		h.nodeStats[index].rollingAvg = float64(h.nodeStats[index].successCount) / float64(total)
	}
}

func (h *HiveRpcNode) logFailureCounts() {
	log.Printf("DEBUG: API Node Failure Counts:")
	log.Printf("| Node | failureCount |")
	for i, addr := range h.addresses {
		log.Printf("| %s | %d |", addr, h.nodeStats[i].failureCount)
	}
}

func (h *HiveRpcNode) logSwitchingNode(currentIndex int, nextIndex int, numNodes int) {
	nextEndpoint := h.addresses[nextIndex]
	log.Printf("DEBUG: Switching to node: %s (index %d)", nextEndpoint, nextIndex)
}

func (h *HiveRpcNode) rpcExecBatchFast(queries []hrpcQuery) ([][]byte, error) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	numNodes := len(h.addresses)
	for i := 0; i < numNodes; i++ {
		index := (h.currentIndex + i) % numNodes
		endpoint := h.addresses[index]

		rpcClient := jsonrpc2client.NewClientWithOpts(endpoint, h.MaxConn, h.MaxBatch)

		var jr2queries jsonrpc2client.RPCRequests
		for j, query := range queries {
			jr2query := &jsonrpc2client.RpcRequest{Method: query.method, JsonRpc: "2.0", Id: j, Params: query.params}
			jr2queries = append(jr2queries, jr2query)
		}

		resps, err := rpcClient.CallBatchFast(jr2queries)
		if err != nil {
			if enableLogging {
				log.Printf("rpcExecBatchFast failed for endpoint %s (index %d): %v", endpoint, index, err)
			}
			h.nodeStats[index].failureCount++
			h.updateRollingAvg(index)
			if enableLogging {
				h.logFailureCounts()
				nextIndex := (h.currentIndex + i + 1) % numNodes
				h.logSwitchingNode(index, nextIndex, numNodes)
			}
			continue
		}

		// Check if any response has error or bad data
		hasError := false
		for _, respBytes := range resps {
			if len(respBytes) == 0 {
				hasError = true
				break
			}
		}
		if hasError {
			if enableLogging {
				log.Printf("rpcExecBatchFast received empty response(s) from endpoint %s (index %d)", endpoint, index)
			}
			h.nodeStats[index].failureCount++
			h.updateRollingAvg(index)
			if enableLogging {
				h.logFailureCounts()
				nextIndex := (h.currentIndex + i + 1) % numNodes
				h.logSwitchingNode(index, nextIndex, numNodes)
			}
			continue
		}

		// Success
		h.nodeStats[index].successCount++
		h.updateRollingAvg(index)
		h.currentIndex = index

		var batchResult [][]byte
		batchResult = append(batchResult, resps...)
		return batchResult, nil
	}

	return nil, errors.New("all API nodes failed")
}

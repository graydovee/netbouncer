// Package history stores bounded, pre-aggregated traffic in disposable SQLite shards.
package history

import "context"

const (
	Minute     int64 = 60
	TenMinutes int64 = 600
	Hour       int64 = 3600
)
const (
	DimTotal = iota
	DimIP
	DimProto
	DimPort
	DimIPProto
	DimIPPort
)

type Delta struct {
	IP                                       string
	Proto                                    string
	Port                                     int
	BytesIn, BytesOut, PacketsIn, PacketsOut uint64
	LastSeen                                 int64
}
type Batch struct {
	ID     string
	TS     int64
	Deltas []Delta
}
type Point struct {
	TS         int64  `json:"ts"`
	IP         string `json:"ip,omitempty"`
	Proto      string `json:"proto,omitempty"`
	Port       int    `json:"port"`
	BytesIn    uint64 `json:"bytes_in"`
	BytesOut   uint64 `json:"bytes_out"`
	PacketsIn  uint64 `json:"packets_in"`
	PacketsOut uint64 `json:"packets_out"`
	BytesSum   uint64 `json:"bytes_sum,omitempty"`
	LastSeen   int64  `json:"last_seen,omitempty"`
}
type Gap struct {
	Start  int64  `json:"start"`
	End    int64  `json:"end"`
	Reason string `json:"reason"`
}
type Meta struct {
	Start          int64 `json:"start"`
	End            int64 `json:"end"`
	Bucket         int64 `json:"bucket"`
	Freshness      int64 `json:"freshness"`
	AvailableStart int64 `json:"available_start"`
	Gaps           []Gap `json:"gaps"`
}
type Result struct {
	Items []Point `json:"items"`
	Meta  Meta    `json:"meta"`
}
type Query struct {
	Start, End, Bucket int64
	IP, Proto          string
	Port               int    // -1 means all ports
	Kind               string // history, ip_top, ports, port_top, protocols
	Limit              int
}
type ShardStatus struct {
	Name       string `json:"name"`
	Resolution int64  `json:"resolution"`
	Start      int64  `json:"start"`
	End        int64  `json:"end"`
	Bytes      int64  `json:"bytes"`
	Watermark  int64  `json:"watermark"`
}
type Status struct {
	SchemaVersion  int           `json:"schema_version"`
	Bytes          int64         `json:"bytes"`
	BudgetBytes    int64         `json:"budget_bytes"`
	FreeBytes      uint64        `json:"free_bytes"`
	ReserveBytes   uint64        `json:"reserve_bytes"`
	AvailableStart int64         `json:"available_start"`
	Freshness      int64         `json:"freshness"`
	Paused         bool          `json:"paused"`
	LastError      string        `json:"last_error,omitempty"`
	WriteFailures  uint64        `json:"write_failures"`
	Gaps           []Gap         `json:"gaps"`
	Shards         []ShardStatus `json:"shards"`
}
type Options struct {
	Dir          string
	BudgetBytes  int64
	ReserveBytes uint64
	// FreeSpace is injectable for failure testing. nil uses statfs.
	FreeSpace func(string) (uint64, error)
}
type Reader interface {
	Query(context.Context, Query) (Result, error)
}

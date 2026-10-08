package core

import "time"

// HistoryDelta is an independent packet increment, not a difference of disposable live counters.
type HistoryDelta struct {
	TS                                       int64
	IP, Proto                                string
	Port                                     int
	BytesIn, BytesOut, PacketsIn, PacketsOut uint64
	LastSeen                                 int64
}
type deltaKey struct {
	ts        int64
	ip, proto string
	port      int
}

// At most 32MiB of capture increments plus 32MiB of writer retries are retained.
const maxCaptureDeltas = (32 << 20) / 256

func (m *Monitor) captureDelta(now time.Time, ip, proto string, port uint16, bytes uint64, sent bool) {
	if !m.historyEnabled || isIPExcluded(ip, m.excludeSubnets) {
		return
	}
	if m.historyDeltas == nil {
		m.historyDeltas = map[deltaKey]*HistoryDelta{}
	}
	ts := now.Unix() / 60 * 60
	k := deltaKey{ts, ip, proto, int(port)}
	d := m.historyDeltas[k]
	if d == nil {
		if len(m.historyDeltas) >= maxCaptureDeltas {
			if len(m.historyLost) == 0 || m.historyLost[len(m.historyLost)-1] != ts {
				m.historyLost = append(m.historyLost, ts)
				if len(m.historyLost) > 360 {
					m.historyLost = m.historyLost[len(m.historyLost)-360:]
				}
			}
			return
		}
		d = &HistoryDelta{TS: ts, IP: ip, Proto: proto, Port: int(port)}
		m.historyDeltas[k] = d
	}
	if sent {
		d.BytesOut += bytes
		d.PacketsOut++
	} else {
		d.BytesIn += bytes
		d.PacketsIn++
	}
	d.LastSeen = now.Unix()
}

// DrainHistory transfers ownership of closed minute buckets, even if the IP was evicted.
func (m *Monitor) DrainHistory(before int64) ([]HistoryDelta, []int64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	out := make([]HistoryDelta, 0, len(m.historyDeltas))
	lost := []int64{}
	for k, d := range m.historyDeltas {
		if k.ts < before {
			out = append(out, *d)
			delete(m.historyDeltas, k)
		}
	}
	keep := m.historyLost[:0]
	for _, ts := range m.historyLost {
		if ts < before {
			lost = append(lost, ts)
		} else {
			keep = append(keep, ts)
		}
	}
	m.historyLost = keep
	return out, lost
}
func (m *Monitor) GetSummaryStats() map[string]*TrafficStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	out := make(map[string]*TrafficStats, len(m.stats))
	for ip, st := range m.stats {
		if isIPExcluded(ip, m.excludeSubnets) {
			continue
		}
		out[ip] = &TrafficStats{RemoteIP: ip, LocalIP: st.localIP, BytesRecv: st.bytesRecv, BytesSent: st.bytesSent, PacketsRecv: st.packetsRecv, PacketsSent: st.packetsSent, BytesRecvPerSec: st.recvWindow.getRate(), BytesSentPerSec: st.sentWindow.getRate(), FirstSeen: st.firstSeen, LastSeen: st.lastSeen, Connections: st.connections, Protocols: map[string]*ProtoPortStats{}}
		for proto, p := range st.protocols {
			out[ip].Protocols[proto] = p.toProtoPortStats()
		}
	}
	return out
}
func (m *Monitor) GetIPStats(ip string) *TrafficStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if isIPExcluded(ip, m.excludeSubnets) {
		return nil
	}
	if st := m.stats[ip]; st != nil {
		return st.toTrafficStats()
	}
	return nil
}

// PortCounters copies only port counters, without live packet windows or IP payloads.
func (m *Monitor) PortCounters() map[string]*TrafficStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	out := make(map[string]*TrafficStats, len(m.stats))
	for ip, st := range m.stats {
		if isIPExcluded(ip, m.excludeSubnets) {
			continue
		}
		cp := &TrafficStats{RemoteIP: ip, Ports: map[string]map[uint16]*ProtoPortStats{}}
		for proto, ports := range st.ports {
			pm := map[uint16]*ProtoPortStats{}
			for port, p := range ports {
				pm[port] = p.toProtoPortStats()
			}
			cp.Ports[proto] = pm
		}
		out[ip] = cp
	}
	return out
}

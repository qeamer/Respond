package ws

import (
	"testing"

	"github.com/pion/rtp"
)

func makeTestPacket(extCount int) *rtp.Packet {
	p := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    111, // Opus
			SequenceNumber: 1234,
			Timestamp:      900000,
			SSRC:           0xdeadbeef,
		},
		// Typical compressed Opus frame: small payload, frequent packets.
		Payload: make([]byte, 80),
	}

	for i := 0; i < extCount; i++ {
		if err := p.Header.SetExtension(uint8(i+1), []byte{0x01, 0x02, 0x03, 0x04}); err != nil {
			panic(err)
		}
	}

	return p
}

func cloneRTPOld(pkt *rtp.Packet) *rtp.Packet {
	if pkt == nil {
		return nil
	}
	raw, err := pkt.Marshal()
	if err != nil {
		return nil
	}
	dup := &rtp.Packet{}
	if err := dup.Unmarshal(raw); err != nil {
		return nil
	}
	return dup
}

func BenchmarkCloneRTP_Old_NoExtensions(b *testing.B) {
	pkt := makeTestPacket(0)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = cloneRTPOld(pkt)
	}
}

func BenchmarkCloneRTP_New_NoExtensions(b *testing.B) {
	pkt := makeTestPacket(0)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = cloneRTP(pkt)
	}
}

func BenchmarkCloneRTP_Old_WithExtensions(b *testing.B) {
	pkt := makeTestPacket(2)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = cloneRTPOld(pkt)
	}
}

func BenchmarkCloneRTP_New_WithExtensions(b *testing.B) {
	pkt := makeTestPacket(2)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = cloneRTP(pkt)
	}
}

func BenchmarkFanout_Old_5Listeners(b *testing.B) {
	pkt := makeTestPacket(0)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for range 5 {
			_ = cloneRTPOld(pkt)
		}
	}
}

func BenchmarkFanout_New_5Listeners(b *testing.B) {
	pkt := makeTestPacket(0)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for range 5 {
			_ = cloneRTP(pkt)
		}
	}
}

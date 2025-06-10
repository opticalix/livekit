package sfu

import (
	"github.com/livekit/protocol/logger"
	"github.com/pion/rtp"
)

const (
	// RTP 包最大大小（考虑 IP 和 UDP 头部）
	MaxRTPPacketSize = 1200
	// NAL 单元起始码长度
	NALStartCodeLength = 4
)

// RTPPacketizer 负责将完整帧分片成 RTP 包
type RTPPacketizer struct {
	logger logger.Logger
	ssrc   uint32
	pt     uint8
}

// NewRTPPacketizer 创建新的 RTP 包分片器
func NewRTPPacketizer(logger logger.Logger, ssrc uint32, pt uint8) *RTPPacketizer {
	return &RTPPacketizer{
		logger: logger,
		ssrc:   ssrc,
		pt:     pt,
	}
}

// Packetize 将完整帧分片成 RTP 包
func (p *RTPPacketizer) Packetize(frame []byte, timestamp uint32) ([]*rtp.Packet, error) {
	var packets []*rtp.Packet
	sequenceNumber := uint16(0)

	// 查找所有 NAL 单元
	nalUnits := p.findNALUnits(frame)
	if len(nalUnits) == 0 {
		return nil, nil
	}

	// 处理每个 NAL 单元
	for i, nal := range nalUnits {
		// 如果 NAL 单元太大，需要分片
		if len(nal) > MaxRTPPacketSize {
			fragments := p.fragmentNAL(nal, sequenceNumber, timestamp, i == len(nalUnits)-1)
			packets = append(packets, fragments...)
			sequenceNumber += uint16(len(fragments))
		} else {
			// 单个 RTP 包
			packet := &rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					Padding:        false,
					Extension:      false,
					Marker:         i == len(nalUnits)-1,
					PayloadType:    p.pt,
					SequenceNumber: sequenceNumber,
					Timestamp:      timestamp,
					SSRC:          p.ssrc,
				},
				Payload: nal,
			}
			packets = append(packets, packet)
			sequenceNumber++
		}
	}

	return packets, nil
}

// findNALUnits 查找帧中的所有 NAL 单元
func (p *RTPPacketizer) findNALUnits(frame []byte) [][]byte {
	var nalUnits [][]byte
	start := 0

	for i := 0; i < len(frame)-NALStartCodeLength; i++ {
		// 查找 NAL 单元起始码 (0x00 0x00 0x00 0x01)
		if frame[i] == 0 && frame[i+1] == 0 && frame[i+2] == 0 && frame[i+3] == 1 {
			if start < i {
				nalUnits = append(nalUnits, frame[start:i])
			}
			start = i + NALStartCodeLength
		}
	}

	// 添加最后一个 NAL 单元
	if start < len(frame) {
		nalUnits = append(nalUnits, frame[start:])
	}

	return nalUnits
}

// fragmentNAL 将 NAL 单元分片
func (p *RTPPacketizer) fragmentNAL(nal []byte, startSeq uint16, timestamp uint32, isLastNAL bool) []*rtp.Packet {
	var fragments []*rtp.Packet
	nalType := nal[0] & 0x1F
	nalHeader := []byte{(nal[0] & 0xE0) | 28} // FU-A 类型
	nalData := nal[1:]

	// 计算分片数量
	numFragments := (len(nalData) + MaxRTPPacketSize - 2) / (MaxRTPPacketSize - 2)

	for i := 0; i < numFragments; i++ {
		start := i * (MaxRTPPacketSize - 2)
		end := start + (MaxRTPPacketSize - 2)
		if end > len(nalData) {
			end = len(nalData)
		}

		// 创建分片
		fragment := make([]byte, 0, 2+end-start)
		fragment = append(fragment, nalHeader...)

		// 设置 FU 头
		fuHeader := byte(0)
		if i == 0 {
			fuHeader |= 0x80 // Start bit
		}
		if i == numFragments-1 {
			fuHeader |= 0x40 // End bit
		}
		fuHeader |= nalType
		fragment = append(fragment, fuHeader)

		// 添加分片数据
		fragment = append(fragment, nalData[start:end]...)

		// 创建 RTP 包
		packet := &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Padding:        false,
				Extension:      false,
				Marker:         i == numFragments-1 && isLastNAL,
				PayloadType:    p.pt,
				SequenceNumber: startSeq + uint16(i),
				Timestamp:      timestamp,
				SSRC:          p.ssrc,
			},
			Payload: fragment,
		}
		fragments = append(fragments, packet)
	}

	return fragments
} 
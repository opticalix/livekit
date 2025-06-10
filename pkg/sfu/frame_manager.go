package sfu

import (
	"errors"
	"sync"
	"time"

	"github.com/livekit/protocol/logger"
	"github.com/pion/rtp"
)

// H264FrameManager 管理H264帧的收集和完整性检查
type H264FrameManager struct {
	mu sync.Mutex

	// 帧缓冲区
	frameBuffer []byte
	lastSeq     uint16
	lastTS      uint32
	ssrc        uint32
	pt          uint8

	// 帧完整性状态
	isComplete bool
	nalUnits   [][]byte

	// 帧超时处理
	frameTimeout time.Duration
	lastReceive  time.Time

	logger logger.Logger
}

// NewH264FrameManager 创建新的H264帧管理器
func NewH264FrameManager(logger logger.Logger) *H264FrameManager {
	return &H264FrameManager{
		frameTimeout: 100 * time.Millisecond,
		lastReceive:  time.Now(),
		logger:       logger,
	}
}

// AddPacket 添加RTP包到帧管理器
func (m *H264FrameManager) AddPacket(packet *rtp.Packet) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Infow("开始处理RTP包",
		"sequence", packet.SequenceNumber,
		"timestamp", packet.Timestamp,
		"payload_length", len(packet.Payload),
		"marker", packet.Marker)

	// 检查SSRC和PayloadType是否匹配
	if m.ssrc != 0 && m.ssrc != packet.SSRC {
		m.logger.Errorw("SSRC不匹配",
			"expected_ssrc", m.ssrc,
			"received_ssrc", packet.SSRC)
		return errors.New("SSRC mismatch")
	}
	if m.pt != 0 && m.pt != packet.PayloadType {
		m.logger.Errorw("PayloadType不匹配",
			"expected_pt", m.pt,
			"received_pt", packet.PayloadType)
		return errors.New("payload type mismatch")
	}

	// 初始化SSRC和PayloadType
	if m.ssrc == 0 {
		m.ssrc = packet.SSRC
		m.pt = packet.PayloadType
		m.logger.Infow("初始化SSRC和PayloadType",
			"ssrc", m.ssrc,
			"payload_type", m.pt)
	}

	// 检查序列号是否连续
	if m.lastSeq != 0 && packet.SequenceNumber != m.lastSeq+1 {
		m.logger.Warnw("序列号不连续",
			"last_sequence", m.lastSeq,
			"current_sequence", packet.SequenceNumber,
			"gap", packet.SequenceNumber-m.lastSeq)
	}

	// 更新序列号和时间戳
	m.lastSeq = packet.SequenceNumber
	m.lastTS = packet.Timestamp

	// 解析NAL单元
	nalUnits, err := m.parseNALUnits(packet.Payload)
	if err != nil {
		m.logger.Errorw("解析NAL单元失败", err)
		return err
	}

	m.logger.Infow("NAL单元解析结果",
		"nal_units_count", len(nalUnits),
		"total_nal_units", len(m.nalUnits)+len(nalUnits))

	// 添加NAL单元到列表
	m.nalUnits = append(m.nalUnits, nalUnits...)

	// 检查帧是否完整
	m.isComplete = m.checkFrameComplete()

	// 更新最后接收时间
	m.lastReceive = time.Now()

	return nil
}

// GetCompleteFrame 获取完整的帧数据
func (m *H264FrameManager) GetCompleteFrame() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isComplete {
		m.logger.Debugw("帧不完整，无法获取完整帧",
			"nal_units_count", len(m.nalUnits),
			"time_since_last_receive", time.Since(m.lastReceive))
		return nil, errors.New("frame not complete")
	}

	m.logger.Infow("开始合并完整帧",
		"nal_units_count", len(m.nalUnits))

	// 合并所有NAL单元
	var frame []byte
	for i, nal := range m.nalUnits {
		frame = append(frame, nal...)
		m.logger.Debugw("合并NAL单元",
			"nal_index", i,
			"nal_type", nal[0]&0x1F,
			"nal_length", len(nal))
	}

	m.logger.Infow("完整帧合并完成",
		"total_frame_length", len(frame))

	// 重置状态
	m.reset()

	return frame, nil
}

// reset 重置帧管理器状态
func (m *H264FrameManager) reset() {
	m.frameBuffer = nil
	m.nalUnits = nil
	m.isComplete = false
}

// parseNALUnits 解析RTP包中的NAL单元
func (m *H264FrameManager) parseNALUnits(payload []byte) ([][]byte, error) {
	var nalUnits [][]byte
	start := 0

	m.logger.Debugw("开始解析NAL单元",
		"payload_length", len(payload))

	for i := 0; i < len(payload)-4; i++ {
		// 查找NAL单元起始码 (0x00 0x00 0x00 0x01)
		if payload[i] == 0 && payload[i+1] == 0 && payload[i+2] == 0 && payload[i+3] == 1 {
			if start < i {
				nalUnit := payload[start:i]
				nalUnits = append(nalUnits, nalUnit)
				m.logger.Debugw("找到NAL单元",
					"nal_type", nalUnit[0]&0x1F,
					"nal_length", len(nalUnit))
			}
			start = i + 4
		}
	}

	// 添加最后一个NAL单元
	if start < len(payload) {
		lastNAL := payload[start:]
		nalUnits = append(nalUnits, lastNAL)
		m.logger.Debugw("添加最后一个NAL单元",
			"nal_type", lastNAL[0]&0x1F,
			"nal_length", len(lastNAL))
	}

	return nalUnits, nil
}

// checkFrameComplete 检查帧是否完整
func (m *H264FrameManager) checkFrameComplete() bool {
	if len(m.nalUnits) == 0 {
		m.logger.Debugw("没有NAL单元，帧不完整")
		return false
	}

	// 检查是否超时
	if time.Since(m.lastReceive) > m.frameTimeout {
		m.logger.Infow("帧超时，标记为完整",
			"time_since_last_receive", time.Since(m.lastReceive),
			"timeout", m.frameTimeout)
		return true
	}

	// 检查最后一个NAL单元是否包含帧结束标记
	lastNAL := m.nalUnits[len(m.nalUnits)-1]
	if len(lastNAL) > 0 {
		nalType := lastNAL[0] & 0x1F
		m.logger.Debugw("检查最后一个NAL单元",
			"nal_type", nalType,
			"is_frame_end", nalType == 0x0A)
		
		if nalType == 0x0A { // 0x0A 是H264的帧结束NAL单元类型
			m.logger.Infow("检测到帧结束NAL单元")
			return true
		}
	}

	return false
}

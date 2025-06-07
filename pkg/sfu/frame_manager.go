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

	// 检查SSRC和PayloadType是否匹配
	if m.ssrc != 0 && m.ssrc != packet.SSRC {
		return errors.New("SSRC mismatch")
	}
	if m.pt != 0 && m.pt != packet.PayloadType {
		return errors.New("payload type mismatch")
	}

	// 初始化SSRC和PayloadType
	if m.ssrc == 0 {
		m.ssrc = packet.SSRC
		m.pt = packet.PayloadType
	}

	// 检查序列号是否连续
	if m.lastSeq != 0 && packet.SequenceNumber != m.lastSeq+1 {
		m.logger.Warnw("sequence number not continuous", nil,
			"last_sequence", m.lastSeq,
			"current_sequence", packet.SequenceNumber)
	}

	// 更新序列号和时间戳
	m.lastSeq = packet.SequenceNumber
	m.lastTS = packet.Timestamp

	// 解析NAL单元
	nalUnits, err := m.parseNALUnits(packet.Payload)
	if err != nil {
		return err
	}

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
		return nil, errors.New("frame not complete")
	}

	// 合并所有NAL单元
	var frame []byte
	for _, nal := range m.nalUnits {
		frame = append(frame, nal...)
	}

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

	for i := 0; i < len(payload)-4; i++ {
		// 查找NAL单元起始码 (0x00 0x00 0x00 0x01)
		if payload[i] == 0 && payload[i+1] == 0 && payload[i+2] == 0 && payload[i+3] == 1 {
			if start < i {
				nalUnits = append(nalUnits, payload[start:i])
			}
			start = i + 4
		}
	}

	// 添加最后一个NAL单元
	if start < len(payload) {
		nalUnits = append(nalUnits, payload[start:])
	}

	return nalUnits, nil
}

// checkFrameComplete 检查帧是否完整
func (m *H264FrameManager) checkFrameComplete() bool {
	if len(m.nalUnits) == 0 {
		return false
	}

	// 检查是否超时
	if time.Since(m.lastReceive) > m.frameTimeout {
		return true
	}

	// 检查最后一个NAL单元是否包含帧结束标记
	lastNAL := m.nalUnits[len(m.nalUnits)-1]
	if len(lastNAL) > 0 && (lastNAL[0]&0x1F) == 0x0A { // 0x0A 是H264的帧结束NAL单元类型
		return true
	}

	return false
}

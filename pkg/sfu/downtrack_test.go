package sfu

import (
	"testing"

	"github.com/livekit/protocol/livekit"
	"github.com/livekit/protocol/logger"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"

	"github.com/livekit/livekit-server/pkg/processing"
	"github.com/livekit/livekit-server/pkg/sfu/buffer"
	"github.com/livekit/livekit-server/pkg/sfu/mime"
)

// mockReceiver 是一个用于测试的 TrackReceiver 实现
type mockReceiver struct {
	trackID livekit.TrackID
}

func (m *mockReceiver) TrackID() livekit.TrackID {
	return m.trackID
}

func (m *mockReceiver) StreamID() string {
	return "test_stream"
}

func (m *mockReceiver) AddOnReady(f func()) {
	f()
}

func (m *mockReceiver) AddDownTrack(dt TrackSender) error {
	// 测试中不需要实际实现
	return nil
}

func (m *mockReceiver) AddOnCodecStateChange(f func(webrtc.RTPCodecParameters, ReceiverCodecState)) {
	// 测试中不需要实际实现
}

func (m *mockReceiver) Codec() webrtc.RTPCodecParameters {
	return webrtc.RTPCodecParameters{}
}

func (m *mockReceiver) CodecState() ReceiverCodecState {
	return ReceiverCodecStateNormal
}

func (m *mockReceiver) DebugInfo() map[string]interface{} {
	return map[string]interface{}{
		"trackID": m.trackID,
	}
}

func (m *mockReceiver) DeleteDownTrack(subID livekit.ParticipantID) {
	// 测试中不需要实际实现
}

func (m *mockReceiver) GetAudioLevel() (float64, bool) {
	return 0, false
}

func (m *mockReceiver) GetDownTracks() []TrackSender {
	return nil
}

func (m *mockReceiver) GetLayeredBitrate() ([]int32, Bitrates) {
	return nil, Bitrates{}
}

func (m *mockReceiver) GetPrimaryReceiverForRed() TrackReceiver {
	return nil
}

func (m *mockReceiver) GetRedReceiver() TrackReceiver {
	return nil
}

func (m *mockReceiver) GetTemporalLayerFpsForSpatial(spatialLayer int32) []float32 {
	return nil
}

func (m *mockReceiver) GetTrackStats() *livekit.RTPStats {
	return nil
}

func (m *mockReceiver) HeaderExtensions() []webrtc.RTPHeaderExtensionParameter {
	return nil
}

func (m *mockReceiver) IsClosed() bool {
	return false
}

func (m *mockReceiver) Mime() mime.MimeType {
	return mime.MimeTypeVP8
}

func (m *mockReceiver) ReadRTP(buf []byte, layer uint8, esn uint64) (int, error) {
	return 0, nil
}

func (m *mockReceiver) SendPLI(layer int32, force bool) {
	// 测试用 mock，无需实现
}

func (m *mockReceiver) SetMaxExpectedSpatialLayer(layer int32) {
	// 测试用 mock，无需实现
}

func (m *mockReceiver) SetUpTrackPaused(paused bool) {
	// 测试用 mock，无需实现
}

func (m *mockReceiver) TrackInfo() *livekit.TrackInfo {
	return nil
}

func (m *mockReceiver) UpdateTrackInfo(ti *livekit.TrackInfo) {
	// 测试用 mock，无需实现
}

func TestDownTrackFrameProcessor(t *testing.T) {
	// 创建测试用的 logger
	testLogger := logger.GetLogger()

	// 创建测试用的 codec
	codec := webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeVP8,
			ClockRate:   90000,
			Channels:    0,
			SDPFmtpLine: "",
		},
		PayloadType: 96,
	}

	// 创建 mock receiver
	mockRecv := &mockReceiver{
		trackID: "test_track",
	}

	// 创建 DownTrack 参数
	params := DowntrackParams{
		Codecs:   []webrtc.RTPCodecParameters{codec},
		Source:   livekit.TrackSource_CAMERA,
		Logger:   testLogger,
		StreamID: "test_stream",
		SubID:    "test_sub",
		MaxTrack: 1,
		Pacer:    nil,
		Trailer:  nil,
		Receiver: mockRecv,
		RTCPWriter: func([]rtcp.Packet) error {
			return nil
		},
	}

	// 创建 DownTrack
	dt, err := NewDownTrack(params)
	require.NoError(t, err)
	require.NotNil(t, dt)

	// 创建测试用的 RTP 包
	rtpPacket := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Padding:        false,
			Extension:      false,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 12345,
			Timestamp:      67890,
			SSRC:           123456,
		},
		Payload: []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05},
	}

	// 创建 ExtPacket
	extPkt := &buffer.ExtPacket{
		Packet:            rtpPacket,
		KeyFrame:          true,
		ExtSequenceNumber: 12345,
		ExtTimestamp:      67890,
	}

	// 测试 WriteRTP
	err = dt.WriteRTP(extPkt, 0)
	require.NoError(t, err)

	// 验证 frameProcessor 是否被正确调用
	// 注意：由于 SimpleProcessor 的实现是简单的日志记录和返回原始数据
	// 我们可以通过检查日志来验证它是否被调用
	// 在实际应用中，您可能需要添加更多的验证逻辑
}

func TestDownTrackFrameProcessorWithCustomProcessor(t *testing.T) {
	// 创建自定义的 FrameProcessor
	customProcessor := &testFrameProcessor{
		logger: logger.GetLogger(),
	}

	// 创建测试用的 codec
	codec := webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeVP8,
			ClockRate:   90000,
			Channels:    0,
			SDPFmtpLine: "",
		},
		PayloadType: 96,
	}

	// 创建 mock receiver
	mockRecv := &mockReceiver{
		trackID: "test_track",
	}

	// 创建 DownTrack 参数
	params := DowntrackParams{
		Codecs:   []webrtc.RTPCodecParameters{codec},
		Source:   livekit.TrackSource_CAMERA,
		Logger:   logger.GetLogger(),
		StreamID: "test_stream",
		SubID:    "test_sub",
		MaxTrack: 1,
		Pacer:    nil,
		Trailer:  nil,
		Receiver: mockRecv,
		RTCPWriter: func([]rtcp.Packet) error {
			return nil
		},
	}

	// 创建 DownTrack
	dt, err := NewDownTrack(params)
	require.NoError(t, err)
	require.NotNil(t, dt)

	// 替换默认的 frameProcessor
	dt.frameProcessor = customProcessor

	// 创建 forwarder
	forwarder := NewForwarder(webrtc.RTPCodecTypeVideo, logger.GetLogger(), false, nil)
	dt.forwarder = forwarder

	// 创建测试用的 RTP 包
	rtpPacket := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Padding:        false,
			Extension:      false,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 12345,
			Timestamp:      67890,
			SSRC:           123456,
		},
		Payload: []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05},
	}

	// 创建 ExtPacket
	extPkt := &buffer.ExtPacket{
		Packet:            rtpPacket,
		KeyFrame:          true,
		ExtSequenceNumber: 12345,
		ExtTimestamp:      67890,
	}

	// 设置 DownTrack 的类型为视频
	dt.kind = webrtc.RTPCodecTypeVideo

	// 设置 writable 为 true
	dt.writable.Store(true)

	// 测试 WriteRTP
	err = dt.WriteRTP(extPkt, 0)
	require.NoError(t, err)

	// 验证自定义处理器是否被调用
	require.True(t, customProcessor.processRTPCalled)
	require.True(t, customProcessor.processFrameCalled)
}

// testFrameProcessor 是一个用于测试的自定义 FrameProcessor 实现
type testFrameProcessor struct {
	logger             logger.Logger
	processRTPCalled   bool
	processFrameCalled bool
}

func (p *testFrameProcessor) ProcessRTP(packet *rtp.Packet) (*processing.ProcessResponse, error) {
	p.processRTPCalled = true
	return &processing.ProcessResponse{
		Data:      packet.Payload,
		Timestamp: packet.Timestamp,
	}, nil
}

func (p *testFrameProcessor) ProcessFrame(req *processing.ProcessRequest) (*processing.ProcessResponse, error) {
	p.processFrameCalled = true
	return &processing.ProcessResponse{
		Data:      req.RawFrame,
		Timestamp: req.Timestamp,
	}, nil
}

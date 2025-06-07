package processing

import (
	// "errors"

	"github.com/livekit/protocol/logger"
	"github.com/pion/rtp"
)

type FrameProcessor interface {
	ProcessFrame(req *ProcessRequest) (*ProcessResponse, error)
	ProcessRTP(packet *rtp.Packet) (*ProcessResponse, error)
}

type ProcessRequest struct {
	RawFrame     []byte
	Timestamp    uint32
	OutputFormat OutputFormat
	Params       ProcessingParams
}

type ProcessingParams struct {
	Disparity   float32
	PopoutRatio float32
	TargetRes   Resolution
}

type ConfigManager interface {
	UpdateConfig(cfg RuntimeConfig) error
	GetCurrentConfig() RuntimeConfig
}

type RuntimeConfig struct {
	MaxFPS           int
	DefaultDisparity float32
	OutputFormat     OutputFormat
	HardwareAccel    bool
}

type Resolution struct {
	Width  int
	Height int
}

type OutputFormat int

const (
	Format2D OutputFormat = iota
	Format3D
)

// ProcessResponse 定义处理响应结构体
type ProcessResponse struct {
	Data      []byte
	Timestamp uint32
}

// 实现示例
type SimpleProcessor struct {
	logger logger.Logger
}

func NewSimpleProcessor(logger logger.Logger) *SimpleProcessor {
	return &SimpleProcessor{
		logger: logger,
	}
}

func (p *SimpleProcessor) ProcessFrame(req *ProcessRequest) (*ProcessResponse, error) {
	// 简单颜色翻转处理
	rawLen := len(req.RawFrame)
	p.logger.Infow("simple process frame", "frame length", rawLen)

	// targetRes := req.Params.TargetRes
	// width, height := targetRes.Width, targetRes.Height
	// ySize := width * height
	// uvSize := (width/2) * (height/2)
	// expectedSize := ySize + uvSize*2
	// p.logger.Infow("simple process frame", "expectedSize", expectedSize)

	// if rawLen != expectedSize {
	// 	p.logger.Warnw("invalid frame length", errors.New("raw frame length invalid"))
	// 	return nil, errors.New("invalid frame format")
	// }

	processed := make([]byte, rawLen)
	for i := 0; i < rawLen; i++ {
		processed[i] = req.RawFrame[i] / 2
	}

	// yPlane := req.RawFrame[:ySize]
	// uPlane := req.RawFrame[ySize : ySize+uvSize]
	// vPlane := req.RawFrame[ySize+uvSize:]

	// 仅反转亮度平面（黑白负片效果）
	// for i := 0; i < ySize; i++ {
	// 	processed[i] = 255 - yPlane[i]
	// }
	// copy(processed[ySize:], req.RawFrame[ySize:])

	return &ProcessResponse{
		Data:      processed,
		Timestamp: req.Timestamp,
	}, nil
}

func (p *SimpleProcessor) ProcessRTP(packet *rtp.Packet) (*ProcessResponse, error) {
	// 简单处理RTP包
	p.logger.Infow("simple process RTP", "payload length", len(packet.Payload))

	// 返回原始数据
	return &ProcessResponse{
		Data:      packet.Payload,
		Timestamp: packet.Timestamp,
	}, nil
}

type DefaultProcessor struct {
	configMgr ConfigManager
}

func NewDefaultProcessor(configMgr ConfigManager) *DefaultProcessor {
	return &DefaultProcessor{
		configMgr: configMgr,
	}
}

func (p *DefaultProcessor) ProcessFrame(req *ProcessRequest) (*ProcessResponse, error) {
	cfg := p.configMgr.GetCurrentConfig()

	// 实现2D转3D处理逻辑
	processed := convertTo3D(req.RawFrame, cfg)

	return &ProcessResponse{
		Data:      processed,
		Timestamp: req.Timestamp,
	}, nil
}

func convertTo3D(frame []byte, cfg RuntimeConfig) []byte {
	// 实现具体的转换逻辑
	return frame
}

// VideoFrameProcessor 视频帧处理器
type VideoFrameProcessor struct {
	logger      logger.Logger
	ffmpeg      *FFmpegProcessor
	frameBuffer []byte
}

func NewVideoFrameProcessor(logger logger.Logger, width, height int) *VideoFrameProcessor {
	return &VideoFrameProcessor{
		logger:      logger,
		ffmpeg:      NewFFmpegProcessor(width, height),
		frameBuffer: make([]byte, 0),
	}
}

// ProcessRTP 处理RTP包
func (p *VideoFrameProcessor) ProcessRTP(rtpPacket *rtp.Packet) (*ProcessResponse, error) {
	// 将RTP包添加到帧缓冲区
	p.frameBuffer = append(p.frameBuffer, rtpPacket.Payload...)

	// 检查是否是完整帧
	if !p.isCompleteFrame() {
		return nil, nil
	}

	// 解码H264帧
	yuvFrame, err := p.ffmpeg.DecodeH264(p.frameBuffer)
	if err != nil {
		p.logger.Errorw("failed to decode H264 frame", err)
		return nil, err
	}

	// 处理YUV帧
	processedYUV, err := p.ffmpeg.ProcessYUV(yuvFrame)
	if err != nil {
		p.logger.Errorw("failed to process YUV frame", err)
		return nil, err
	}

	// 重新编码为H264
	encodedFrame, err := p.ffmpeg.EncodeH264(processedYUV)
	if err != nil {
		p.logger.Errorw("failed to encode H264 frame", err)
		return nil, err
	}

	// 清空帧缓冲区
	p.frameBuffer = p.frameBuffer[:0]

	return &ProcessResponse{
		Data:      encodedFrame,
		Timestamp: rtpPacket.Timestamp,
	}, nil
}

// isCompleteFrame 检查是否是完整帧
func (p *VideoFrameProcessor) isCompleteFrame() bool {
	// 检查帧结束标记
	if len(p.frameBuffer) < 4 {
		return false
	}

	// 检查NAL单元结束标记
	return p.frameBuffer[len(p.frameBuffer)-4] == 0x00 &&
		p.frameBuffer[len(p.frameBuffer)-3] == 0x00 &&
		p.frameBuffer[len(p.frameBuffer)-2] == 0x00 &&
		p.frameBuffer[len(p.frameBuffer)-1] == 0x01
}

package processing

import (
	"bytes"
	"fmt"
	"os/exec"
)

// FFmpegProcessor 使用FFmpeg进行视频处理
type FFmpegProcessor struct {
	width  int
	height int
}

// NewFFmpegProcessor 创建新的FFmpeg处理器
func NewFFmpegProcessor(width, height int) *FFmpegProcessor {
	return &FFmpegProcessor{
		width:  width,
		height: height,
	}
}

// DecodeH264 解码H264帧为YUV
func (p *FFmpegProcessor) DecodeH264(h264Data []byte) ([]byte, error) {
	cmd := exec.Command("ffmpeg",
		"-f", "h264",
		"-i", "pipe:0",
		"-f", "rawvideo",
		"-pix_fmt", "yuv420p",
		"-",
	)

	cmd.Stdin = bytes.NewReader(h264Data)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg decode error: %v", err)
	}

	return out.Bytes(), nil
}

// EncodeH264 将YUV帧编码为H264
func (p *FFmpegProcessor) EncodeH264(yuvData []byte) ([]byte, error) {
	cmd := exec.Command("ffmpeg",
		"-f", "rawvideo",
		"-pix_fmt", "yuv420p",
		"-s", fmt.Sprintf("%dx%d", p.width, p.height),
		"-i", "pipe:0",
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-tune", "zerolatency",
		"-f", "h264",
		"-",
	)

	cmd.Stdin = bytes.NewReader(yuvData)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg encode error: %v", err)
	}

	return out.Bytes(), nil
}

// ProcessYUV 处理YUV帧
func (p *FFmpegProcessor) ProcessYUV(yuvData []byte) ([]byte, error) {
	// 这里可以实现具体的YUV帧处理逻辑
	// 例如：调整亮度、对比度、应用滤镜等
	return yuvData, nil
}

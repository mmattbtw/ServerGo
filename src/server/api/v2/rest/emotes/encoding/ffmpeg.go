package encoding

import "os"

type ffmpeg struct{}

var FFmpeg = ffmpeg{}

func (ffmpeg) Encode() (*os.File, error) {
	return nil, nil
}

package datastructure

import (
	"fmt"
)

type emoteUtil struct{}

func (*emoteUtil) GetFilesMeta(fileDir string) [][]string {
	// Define sizes to be generated
	// File path, emote size, emote width/height, quality factor
	return [][]string{
		{fmt.Sprintf("%s/1x", fileDir), "1x", "96x32", "100"},
		{fmt.Sprintf("%s/2x", fileDir), "2x", "144x48", "90"},
		{fmt.Sprintf("%s/3x", fileDir), "3x", "228x76", "90"},
		{fmt.Sprintf("%s/4x", fileDir), "4x", "384x128", "95"},
	}
}

var EmoteUtil emoteUtil

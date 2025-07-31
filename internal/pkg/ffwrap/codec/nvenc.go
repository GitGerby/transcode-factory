package codec

import "fmt"

func buildNvencHevc(crf int) []string {
	return []string{
		"-pix_fmt", "p010le",
		"-c:v", "hevc_nvenc",
		"-rc", "1",
		"-cq", fmt.Sprintf("%d", crf),
		"-profile:v", "1",
		"-tier", "1",
		"-spatial_aq", "1",
		"-temporal_aq", "1",
		"-preset", "1",
		"-b_ref_mode", "2",
	}
}

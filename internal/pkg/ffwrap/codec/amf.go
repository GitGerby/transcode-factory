package codec

func buildAv1Amf() []string {
	return []string{
		"-c:v", "av1_amf",
		"-quality", "quality",
		"-vbaq", "true",
		"-bitdepth", "10",
		"-pix_fmt", "yuv420p10le",
	}
}

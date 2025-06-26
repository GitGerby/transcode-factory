package codec

const (
	SideDataTypeMastering  = "Mastering display metadata"
	SideDataTypeLightLevel = "Content light level metadata"
)

type ColorCoords struct {
	Coordinates string
}

type ColorInfo struct {
	Color_space     string          `json:"color_space"`
	Color_primaries string          `json:"color_primaries"`
	Color_transfer  string          `json:"color_transfer"`
	Side_data_list  []ColorSideInfo `json:"side_data_list"`
}

type ColorSideInfo struct {
	Side_data_type string `json:"side_data_type"`
	Red_x          string `json:"red_x"`
	Red_y          string `json:"red_y"`
	Green_x        string `json:"green_x"`
	Green_y        string `json:"green_y"`
	Blue_x         string `json:"Blue_x"`
	Blue_y         string `json:"Blue_y"`
	White_point_x  string `json:"White_point_x"`
	White_point_y  string `json:"White_point_y"`
	Min_luminance  string `json:"Min_luminance"`
	Max_luminance  string `json:"Max_luminance"`
	Max_content    int    `json:"Max_content"`
	Max_average    int    `json:"Max_average"`
}

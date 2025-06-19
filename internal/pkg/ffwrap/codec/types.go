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

// Vars used for tests
var (
	ValidMasteringColorInfo = ColorSideInfo{
		Side_data_type: SideDataTypeMastering,
		Red_x:          "30/100",
		Red_y:          "30/100",
		Green_x:        "40/100",
		Green_y:        "40/100",
		Blue_x:         "20/100",
		Blue_y:         "20/100",
		White_point_x:  "90/100",
		White_point_y:  "100/100",
		Max_luminance:  "100/100",
		Min_luminance:  "0/100",
	}
	MissingMasteringColorInfo = ColorSideInfo{
		Side_data_type: SideDataTypeMastering,
		Red_x:          "30/100",
		Red_y:          "30/100",
		Green_x:        "40/100",
		White_point_x:  "90/100",
		White_point_y:  "100/100",
	}
	NanMasteringColorInfo = ColorSideInfo{
		Side_data_type: SideDataTypeMastering,
		Red_x:          "30/100",
		Red_y:          "30/100",
		Green_x:        "40/100",
		Green_y:        "40/100",
		Blue_x:         "a/100",
		Blue_y:         "20/100",
		White_point_x:  "90/100",
		White_point_y:  "100/100",
		Max_luminance:  "a/100",
		Min_luminance:  "0/100",
	}
	ValidLightColorInfo = ColorSideInfo{
		Side_data_type: SideDataTypeLightLevel,
		Max_content:    700,
		Max_average:    200,
	}
	MissingLightColorInfo = ColorSideInfo{
		Side_data_type: SideDataTypeLightLevel,
		Max_content:    700,
	}
)

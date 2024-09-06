package blueprint

type SwapfileCustomization struct {
	Path string `json:"path" toml:"path"`
	Size uint64 `json:"size" toml:"size"`
}

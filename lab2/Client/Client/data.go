package Client

type FileInfo struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

const (
	maxSize        int64 = 1 << 40
	maxLenFileName       = 4096
	countArgument        = 4
	buffSize             = 64 * 1024
)

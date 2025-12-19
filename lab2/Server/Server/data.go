package Server

type FileInfo struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

const (
	permissionBits = 0755
	countSameFile  = 100
	buffSize       = 64 * 1024
)

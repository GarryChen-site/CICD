package remote

type InstanceSpec struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	CPU float32 `json:"cpu"`
	Memory float32 `json:"memory"`
	Disk float32 `json:"disk"`
}
package idgen

type Generator interface {
	NextID() (uint64, error)
}

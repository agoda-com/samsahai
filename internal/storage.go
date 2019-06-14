package internal

type Storage interface {
	Load()

	Read() ([]byte, error)

	Save()
}

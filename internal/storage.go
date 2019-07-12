package internal

type StorageController interface {
	// Start starts the storage and keep in-sync with target repository
	Start(shutdown <-chan struct{})

	// Stop stops sync and exit
	Stop()

	// Notify
	//Notify()

	// OnChanged notifies on changed
	OnChanged() <-chan struct{}

	// Read returns file content
	//Read(filepath string) ([]byte, error)
}

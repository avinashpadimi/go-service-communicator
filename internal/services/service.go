package services

// Communicator is an interface that defines the methods for a service that can send messages.
type Communicator interface {
	SendMessage(destination, message string) error
}

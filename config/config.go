package config

// Config interface defines the basic configuration contract
type Config interface {
	GetName() string
	Validate() error
}

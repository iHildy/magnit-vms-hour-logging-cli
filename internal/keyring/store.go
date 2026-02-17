package keyring

import (
	"errors"
	"fmt"

	zk "github.com/zalando/go-keyring"
)

const (
	serviceName = "magnit-vms-hour-logging-cli"
	userKey     = "username"
	passKey     = "password"
)

type Credentials struct {
	Username string
	Password string
}

func SaveCredentials(creds Credentials) error {
	if creds.Username == "" {
		return errors.New("username is required")
	}
	if creds.Password == "" {
		return errors.New("password is required")
	}

	if err := zk.Set(serviceName, userKey, creds.Username); err != nil {
		return fmt.Errorf("save username to keyring: %w", err)
	}
	if err := zk.Set(serviceName, passKey, creds.Password); err != nil {
		return fmt.Errorf("save password to keyring: %w", err)
	}
	return nil
}

func LoadCredentials() (Credentials, error) {
	username, err := zk.Get(serviceName, userKey)
	if err != nil {
		if errors.Is(err, zk.ErrNotFound) {
			return Credentials{}, fmt.Errorf("credentials not found")
		}
		return Credentials{}, fmt.Errorf("read username from keyring: %w", err)
	}

	password, err := zk.Get(serviceName, passKey)
	if err != nil {
		if errors.Is(err, zk.ErrNotFound) {
			return Credentials{}, fmt.Errorf("credentials not found")
		}
		return Credentials{}, fmt.Errorf("read password from keyring: %w", err)
	}

	return Credentials{Username: username, Password: password}, nil
}

func DeleteCredentials() error {
	userErr := zk.Delete(serviceName, userKey)
	passErr := zk.Delete(serviceName, passKey)
	if userErr != nil && !errors.Is(userErr, zk.ErrNotFound) {
		return fmt.Errorf("delete username from keyring: %w", userErr)
	}
	if passErr != nil && !errors.Is(passErr, zk.ErrNotFound) {
		return fmt.Errorf("delete password from keyring: %w", passErr)
	}
	return nil
}

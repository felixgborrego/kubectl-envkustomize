package pkg

import "path/filepath"
import "sigs.k8s.io/kustomize/kyaml/filesys"

// EnvAwareFileSystem is a wrapper around a Kustomize FileSystem that handles environment variable substitution.
// it delegates all operations to the original Kustomize FileSystem, but reads files with environment variable substitution.
type EnvAwareFileSystem struct {
	wrapped               filesys.FileSystem
	handleEnvSubstitution func([]byte) ([]byte, error)
}

// NewEnvAwareFileSystem creates a new EnvAwareFileSystem that wraps the provided FileSystem.
func NewEnvAwareFileSystem(fs filesys.FileSystem, handleEnvSubstitution func([]byte) ([]byte, error)) *EnvAwareFileSystem {
	return &EnvAwareFileSystem{
		wrapped:               fs,
		handleEnvSubstitution: handleEnvSubstitution,
	}
}

// Create delegates to the wrapped FileSystem.
func (e *EnvAwareFileSystem) Create(path string) (filesys.File, error) {
	return e.wrapped.Create(path)
}

// Mkdir delegates to the wrapped FileSystem.
func (e *EnvAwareFileSystem) Mkdir(path string) error {
	return e.wrapped.Mkdir(path)
}

// MkdirAll delegates to the wrapped FileSystem.
func (e *EnvAwareFileSystem) MkdirAll(path string) error {
	return e.wrapped.MkdirAll(path)
}

// RemoveAll delegates to the wrapped FileSystem.
func (e *EnvAwareFileSystem) RemoveAll(path string) error {
	return e.wrapped.RemoveAll(path)
}

// Open delegates to the wrapped FileSystem.
func (e *EnvAwareFileSystem) Open(path string) (filesys.File, error) {
	return e.wrapped.Open(path)
}

// IsDir delegates to the wrapped FileSystem.
func (e *EnvAwareFileSystem) IsDir(path string) bool {
	return e.wrapped.IsDir(path)
}

// ReadDir delegates to the wrapped FileSystem.
func (e *EnvAwareFileSystem) ReadDir(path string) ([]string, error) {
	return e.wrapped.ReadDir(path)
}

// CleanedAbs delegates to the wrapped FileSystem.
func (e *EnvAwareFileSystem) CleanedAbs(path string) (filesys.ConfirmedDir, string, error) {
	return e.wrapped.CleanedAbs(path)
}

// Exists delegates to the wrapped FileSystem.
func (e *EnvAwareFileSystem) Exists(path string) bool {
	return e.wrapped.Exists(path)
}

// Glob delegates to the wrapped FileSystem.
func (e *EnvAwareFileSystem) Glob(pattern string) ([]string, error) {
	return e.wrapped.Glob(pattern)
}

// WriteFile delegates to the wrapped FileSystem.
func (e *EnvAwareFileSystem) WriteFile(path string, data []byte) error {
	return e.wrapped.WriteFile(path, data)
}

// Walk delegates to the wrapped FileSystem.
func (e *EnvAwareFileSystem) Walk(path string, walkFn filepath.WalkFunc) error {
	return e.wrapped.Walk(path, walkFn)
}

// ReadFile reads the file and applies environment variable substitution before returning the contents.
func (e *EnvAwareFileSystem) ReadFile(path string) ([]byte, error) {
	data, err := e.wrapped.ReadFile(path)
	if err != nil {
		return nil, err
	}

	result, err := e.handleEnvSubstitution(data)
	if err != nil {
		return nil, err
	}
	// Apply environment variable substitution using the injected function.
	return result, nil
}

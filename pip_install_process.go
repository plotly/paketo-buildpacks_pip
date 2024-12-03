package pip

import (
	"bytes"
	"fmt"
	"os"

	"github.com/paketo-buildpacks/packit/v2/pexec"
)

//go:generate faux --interface Executable --output fakes/executable.go

// Executable defines the interface for invoking an executable.
type Executable interface {
	Execute(pexec.Execution) error
}

// PipInstallProcess implements the InstallProcess interface.
type PipInstallProcess struct {
	executable Executable
}

// NewPipInstallProcess creates an instance of the PipInstallProcess given an Executable that runs `python`.
func NewPipInstallProcess(executable Executable) PipInstallProcess {
	return PipInstallProcess{
		executable: executable,
	}
}

// Execute installs the pip binary from source code located in the given srcPath into the a layer path designated by targetLayerPath.
func (p PipInstallProcess) Execute(srcPath, targetLayerPath string) error {
	buffer := bytes.NewBuffer(nil)

	err_pip := p.executable.Execute(pexec.Execution{
		// Install pip from source with the pip that comes pre-installed with cpython
		Args: []string{"-m", "pip", "install", srcPath, "--user", "--no-index", fmt.Sprintf("--find-links=%s", srcPath)},
		// Set the PYTHONUSERBASE to ensure that pip is installed to the newly created target layer.
		Env:    append(os.Environ(), fmt.Sprintf("PYTHONUSERBASE=%s", targetLayerPath)),
		Stdout: os.Stdout,
		Stderr: buffer,
	})
	if err_pip != nil {
		return fmt.Errorf("failed to configure pip:\n%s\nerror: %w", buffer.String(), err_pip)
	}

	err_venv := p.executable.Execute(pexec.Execution{
		// Install venv from source with the pip that comes pre-installed with cpython
		Args: []string{"-m", "pip", "install", "venv", "--user", "--no-index", fmt.Sprintf("--find-links=%s", srcPath)},
		// Set the PYTHONUSERBASE to ensure that pip is installed to the newly created target layer.
		Env:    append(os.Environ(), fmt.Sprintf("PYTHONUSERBASE=%s", targetLayerPath)),
		Stdout: os.Stdout,
		Stderr: buffer,
	})

	if err_venv != nil {
		return fmt.Errorf("failed to configure venv:\n%s\nerror: %w", buffer.String(), err_venv)
	}

	err_uv := p.executable.Execute(pexec.Execution{
		// Install venv from source with the pip that comes pre-installed with cpython
		Args: []string{"-m", "pip", "install", "uv", "--user", "--no-index", fmt.Sprintf("--find-links=%s", srcPath)},
		// Set the PYTHONUSERBASE to ensure that pip is installed to the newly created target layer.
		Env:    append(os.Environ(), fmt.Sprintf("PYTHONUSERBASE=%s", targetLayerPath)),
		Stdout: os.Stdout,
		Stderr: buffer,
	})

	if err_uv != nil {
		return fmt.Errorf("failed to configure uv:\n%s\nerror: %w", buffer.String(), err_uv)
	}

	err_venv_setup := p.executable.Execute(pexec.Execution{
		// Install venv from source with the pip that comes pre-installed with cpython
		Args: []string{"-m", "venv", targetLayerPath},
		// Set the PYTHONUSERBASE to ensure that pip is installed to the newly created target layer.
		Env:    append(os.Environ(), fmt.Sprintf("PYTHONUSERBASE=%s", targetLayerPath)),
		Stdout: os.Stdout,
		Stderr: buffer,
	})

	if err_venv_setup != nil {
		return fmt.Errorf("failed to setup venv in targetLayer uv:\n%s\nerror: %w", buffer.String(), err_venv_setup)
	}

	return nil
}

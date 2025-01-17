package truststore

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
)

type linuxStore struct{}

// Platform returns the truststore for the current platform.
func Platform() (Truststore, error) {
	return &linuxStore{}, nil
}

// installCommand describes the command necessary to install
// a root certificate on the current platform.
type installCommand struct {
	rootsPattern string
	command      []string
}

func strategy() (installCommand, error) {
	if pathExists("/etc/pki/ca-trust/source/anchors/") {
		return installCommand{
			rootsPattern: "/etc/pki/ca-trust/source/anchors/%s.pem",
			command:      []string{"update-ca-trust", "extract"},
		}, nil
	} else if pathExists("/usr/local/share/ca-certificates/") {
		return installCommand{
			rootsPattern: "/usr/local/share/ca-certificates/%s.crt",
			command:      []string{"update-ca-certificates"},
		}, nil
	} else if pathExists("/etc/ca-certificates/trust-source/anchors/") {
		return installCommand{
			rootsPattern: "/etc/ca-certificates/trust-source/anchors/%s.crt",
			command:      []string{"trust", "extract-compat"},
		}, nil
	} else if pathExists("/usr/share/pki/trust/anchors") {
		return installCommand{
			rootsPattern: "/usr/share/pki/trust/anchors/%s.pem",
			command:      []string{"update-ca-certificates"},
		}, nil
	}

	return installCommand{}, errors.New("no install strategy available")
}

// Install installs the pem-encoded root certificate at the provided path
// to the system store.
func (i *linuxStore) Install(path string) error {
	pemData, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read root certificate: %v", err)
	}
	c, err := decodeCert(path)
	if err != nil {
		return fmt.Errorf("failed to parse root certificate: %v", err)
	}
	strategy, err := strategy()
	if err != nil {
		return err
	}

	cmd, err := commandWithSudo("tee", fmt.Sprintf(strategy.rootsPattern, strings.Replace("mkcert development CA "+c.SerialNumber.String(), " ", "_", -1)))
	if err != nil {
		return err
	}
	cmd.Stdin = bytes.NewReader(pemData)
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("command %q failed: %v", "tee", err)
	}

	cmd, err = commandWithSudo(strategy.command...)
	if err != nil {
		return err
	}
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("command %q failed: %v", strings.Join(strategy.command, " "), err)
	}
	return nil
}

// Uninstall removes the PEM-encoded certificate at path from the system store.
func (i *linuxStore) Uninstall(path string) error {
	c, err := decodeCert(path)
	if err != nil {
		return fmt.Errorf("failed to parse root certificate: %v", err)
	}
	strategy, err := strategy()
	if err != nil {
		return err
	}
	cmd, err := commandWithSudo("rm", "-f", fmt.Sprintf(strategy.rootsPattern, strings.Replace("mkcert development CA "+c.SerialNumber.String(), " ", "_", -1)))
	if err != nil {
		return err
	}
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("command %q failed: %v", "rm", err)
	}

	// We used to install under non-unique filenames.
	legacyFilename := fmt.Sprintf(strategy.rootsPattern, "mkcert-rootCA")
	if pathExists(legacyFilename) {
		cmd, err := commandWithSudo("rm", "-f", legacyFilename)
		if err != nil {
			return err
		}
		if _, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("command %q failed: %v", "rm (legacy filename)", err)
		}
	}

	if cmd, err = commandWithSudo(strategy.command...); err != nil {
		return err
	}
	if _, err = cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("command %q failed: %v", strings.Join(strategy.command, " "), err)
	}
	return nil
}

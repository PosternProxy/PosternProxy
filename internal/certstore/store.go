// Package certstore validates, encrypts and stages custom TLS certificate pairs.
package certstore

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/posternproxy/posternproxy/internal/security"
)

const MaxPEMSize = 2 << 20

var managedName = regexp.MustCompile(`^[1-9][0-9]*-[0-9a-f]{64}$`)

type Pair struct {
	Certificate []byte
	Key         []byte
}

type Store struct {
	Root string
	Key  [32]byte
}

func Read(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, MaxPEMSize+1))
	if err != nil {
		return nil, fmt.Errorf("read PEM: %w", err)
	}
	if len(b) > MaxPEMSize {
		return nil, fmt.Errorf("PEM exceeds %d bytes", MaxPEMSize)
	}
	return b, nil
}

func Validate(p Pair, domains []string) (*x509.Certificate, error) {
	if len(p.Certificate) > MaxPEMSize || len(p.Key) > MaxPEMSize {
		return nil, fmt.Errorf("certificate material exceeds size limit")
	}
	for kind, raw := range [][]byte{p.Certificate, p.Key} {
		remaining := bytes.TrimSpace(raw)
		blocks := 0
		if len(remaining) == 0 {
			return nil, fmt.Errorf("empty PEM data")
		}
		for len(remaining) > 0 {
			if !bytes.HasPrefix(remaining, []byte("-----BEGIN ")) {
				return nil, fmt.Errorf("invalid data outside PEM block")
			}
			block, rest := pem.Decode(remaining)
			if block == nil {
				return nil, fmt.Errorf("invalid PEM block")
			}
			blocks++
			if kind == 0 && block.Type != "CERTIFICATE" {
				return nil, fmt.Errorf("certificate chain contains an unexpected PEM block")
			}
			if kind == 1 && (blocks != 1 || (block.Type != "PRIVATE KEY" && block.Type != "RSA PRIVATE KEY" && block.Type != "EC PRIVATE KEY")) {
				return nil, fmt.Errorf("expected exactly one unencrypted private-key PEM block")
			}
			remaining = bytes.TrimSpace(rest)
		}
	}
	pair, err := tls.X509KeyPair(p.Certificate, p.Key)
	if err != nil {
		return nil, fmt.Errorf("invalid certificate/key pair: %w", err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}
	serverUsage := len(leaf.ExtKeyUsage) == 0 && len(leaf.UnknownExtKeyUsage) == 0
	for _, usage := range leaf.ExtKeyUsage {
		serverUsage = serverUsage || usage == x509.ExtKeyUsageServerAuth || usage == x509.ExtKeyUsageAny
	}
	if !serverUsage {
		return nil, fmt.Errorf("certificate is not valid for TLS server authentication")
	}
	now := time.Now()
	if now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
		return nil, fmt.Errorf("certificate is outside its validity period")
	}
	for _, domain := range domains {
		if strings.HasPrefix(domain, "*.") {
			covered := false
			for _, name := range leaf.DNSNames {
				covered = covered || strings.EqualFold(domain, name)
			}
			if !covered {
				return nil, fmt.Errorf("certificate does not cover requested wildcard")
			}
		} else if err := leaf.VerifyHostname(domain); err != nil {
			return nil, fmt.Errorf("certificate does not cover requested domain: %w", err)
		}
	}
	return leaf, nil
}

func Name(id int64, p Pair) string {
	h := sha256.New()
	h.Write(p.Certificate)
	h.Write([]byte{0})
	h.Write(p.Key)
	return strconv.FormatInt(id, 10) + "-" + hex.EncodeToString(h.Sum(nil))
}

// Save uses immutable paths so a database rollback never points at overwritten keys.
func (s Store) Save(id int64, p Pair) (string, string, error) {
	if len(p.Certificate) > MaxPEMSize || len(p.Key) > MaxPEMSize {
		return "", "", fmt.Errorf("stored certificate material exceeds size limit")
	}
	certPath := filepath.Join(s.Root, Name(id, p), "cert.pem")
	keyPath := filepath.Join(s.Root, Name(id, p), "key.pem")
	if _, err := os.Lstat(filepath.Dir(certPath)); err == nil {
		existing, err := s.Load(certPath, keyPath)
		if err != nil || !bytes.Equal(existing.Certificate, p.Certificate) || !bytes.Equal(existing.Key, p.Key) {
			return "", "", fmt.Errorf("existing encrypted certificate pair is inconsistent")
		}
		return certPath, keyPath, nil
	} else if !os.IsNotExist(err) {
		return "", "", err
	}
	cert, err := security.Encrypt(s.Key, string(p.Certificate))
	if err != nil {
		return "", "", err
	}
	key, err := security.Encrypt(s.Key, string(p.Key))
	if err != nil {
		return "", "", err
	}
	dir, err := stage(s.Root, Name(id, p), []byte("enc:v1:"+cert), []byte("enc:v1:"+key), 0700, 0600)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem"), nil
}

func (s Store) Load(certPath, keyPath string) (Pair, error) {
	var p Pair
	for n, path := range []string{certPath, keyPath} {
		if err := within(s.Root, path); err != nil {
			return p, err
		}
		if err := noSymlinks(path); err != nil {
			return p, err
		}
		f, err := os.Open(path) // #nosec G304 -- path is within the server's own certificate store, not request input
		if err != nil {
			return p, fmt.Errorf("open required certificate material: %w", err)
		}
		b, readErr := io.ReadAll(io.LimitReader(f, 4*MaxPEMSize+1))
		closeErr := f.Close()
		if readErr != nil {
			return p, readErr
		}
		if closeErr != nil {
			return p, closeErr
		}
		if len(b) > 4*MaxPEMSize {
			return p, fmt.Errorf("stored certificate material exceeds size limit")
		}
		if bytes.HasPrefix(b, []byte("enc:v1:")) {
			plain, err := security.Decrypt(s.Key, strings.TrimPrefix(string(b), "enc:v1:"))
			if err != nil {
				return p, fmt.Errorf("decrypt certificate material: %w", err)
			}
			b = []byte(plain)
		}
		if n == 0 {
			p.Certificate = b
		} else {
			p.Key = b
		}
	}
	return p, nil
}

func StageRuntime(root string, id int64, p Pair) (string, error) {
	if _, err := Validate(p, nil); err != nil {
		return "", err
	}
	return stage(root, Name(id, p), p.Certificate, p.Key, 0750, 0640)
}

func stage(root, name string, cert, key []byte, dirMode, fileMode os.FileMode) (result string, err error) {
	if !managedName.MatchString(name) {
		return "", fmt.Errorf("invalid managed certificate name")
	}
	if err := noSymlinks(root); err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, dirMode); err != nil {
		return "", err
	}
	if dirMode == 0750 {
		if err := runtimeGroup(root); err != nil {
			return "", err
		}
		if err := os.Chmod(root, dirMode); err != nil {
			return "", err
		}
	}
	final := filepath.Join(root, name)
	if info, err := os.Lstat(final); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("unsafe existing certificate directory")
		}
		for n, base := range []string{"cert.pem", "key.pem"} {
			if err := noSymlinks(filepath.Join(final, base)); err != nil {
				return "", err
			}
			actual, err := os.ReadFile(filepath.Join(final, base)) // #nosec G304 -- path is within the server's own certificate store, not request input
			want := cert
			if n == 1 {
				want = key
			}
			if err != nil || !bytes.Equal(actual, want) {
				return "", fmt.Errorf("existing runtime certificate pair is inconsistent")
			}
			if err := os.Chmod(filepath.Join(final, base), fileMode); err != nil {
				return "", err
			}
			if dirMode == 0750 {
				if err := runtimeGroup(filepath.Join(final, base)); err != nil {
					return "", err
				}
			}
		}
		if err := os.Chmod(final, dirMode); err != nil {
			return "", err
		}
		if dirMode == 0750 {
			if err := runtimeGroup(final); err != nil {
				return "", err
			}
		}
		return final, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	tmp, err := os.MkdirTemp(root, ".stage-*")
	if err != nil {
		return "", err
	}
	defer func() {
		if cleanupErr := os.RemoveAll(tmp); cleanupErr != nil && err == nil {
			err = fmt.Errorf("clean staged certificate: %w", cleanupErr)
		}
	}()
	for n, base := range []string{"cert.pem", "key.pem"} {
		data := cert
		if n == 1 {
			data = key
		}
		f, err := os.OpenFile(filepath.Join(tmp, base), os.O_CREATE|os.O_EXCL|os.O_WRONLY, fileMode) // #nosec G304 -- path is within the server's own certificate store, not request input
		if err != nil {
			return "", err
		}
		_, writeErr := f.Write(data)
		modeErr := f.Chmod(fileMode)
		syncErr := f.Sync()
		closeErr := f.Close()
		for _, err := range []error{writeErr, modeErr, syncErr, closeErr} {
			if err != nil {
				return "", err
			}
		}
		if dirMode == 0750 {
			if err := runtimeGroup(filepath.Join(tmp, base)); err != nil {
				return "", err
			}
		}
	}
	if err := os.Chmod(tmp, dirMode); err != nil {
		return "", err
	}
	if dirMode == 0750 {
		if err := runtimeGroup(tmp); err != nil {
			return "", err
		}
	}
	if err := syncDirectory(tmp); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, final); err != nil {
		return "", err
	}
	if err := syncDirectory(root); err != nil {
		return "", err
	}
	return final, nil
}

func PruneLegacy(root string, retained ...map[string]bool) error {
	if err := noSymlinks(root); err != nil {
		return err
	}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if len(retained) > 0 && retained[0][entry.Name()] {
			continue
		}
		id, err := strconv.ParseInt(entry.Name(), 10, 64)
		if err != nil || id <= 0 || entry.Name() != strconv.FormatInt(id, 10) {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if err := noSymlinks(dir); err != nil {
			return err
		}
		for _, base := range []string{"cert.pem", "key.pem"} {
			path := filepath.Join(dir, base)
			if err := within(root, path); err != nil {
				return err
			}
			if err := noSymlinks(path); err != nil {
				return err
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func PruneRuntime(root string, keep map[string]bool) error {
	if err := noSymlinks(root); err != nil {
		return err
	}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !managedName.MatchString(entry.Name()) || keep[entry.Name()] {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if err := noSymlinks(path); err != nil {
			return err
		}
		if err := within(root, path); err != nil {
			return err
		}
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("retire certificate runtime files: %w", err)
		}
	}
	return nil
}

func within(root, target string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absPath, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return fmt.Errorf("certificate path is outside managed storage")
	}
	return nil
}

func noSymlinks(path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for {
		info, err := os.Lstat(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not allowed in managed certificate paths")
		}
		parent := filepath.Dir(path)
		if parent == path {
			return nil
		}
		path = parent
	}
}

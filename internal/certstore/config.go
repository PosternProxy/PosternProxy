package certstore

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

type Material struct {
	ID   int64
	Pair Pair
}

// StageConfig rewrites only managed certificate loaders, retaining other Caddy
// fields. The manifest must account for every file loader and every delivered key.
func StageConfig(raw json.RawMessage, material []Material, root string) (json.RawMessage, map[string]bool, error) {
	var config map[string]json.RawMessage
	if err := json.Unmarshal(raw, &config); err != nil || config == nil {
		return nil, nil, fmt.Errorf("invalid full Caddy configuration")
	}
	var apps, tlsApp, certificates map[string]json.RawMessage
	if err := decodeObject(config["apps"], &apps); err != nil {
		return nil, nil, err
	}
	if err := decodeObject(apps["tls"], &tlsApp); err != nil {
		return nil, nil, err
	}
	if err := decodeObject(tlsApp["certificates"], &certificates); err != nil {
		return nil, nil, err
	}
	type loader struct {
		Certificate string   `json:"certificate"`
		Key         string   `json:"key"`
		Tags        []string `json:"tags,omitempty"`
	}
	var files []loader
	if len(certificates["load_files"]) > 0 {
		if err := json.Unmarshal(certificates["load_files"], &files); err != nil {
			return nil, nil, err
		}
	}
	byName := make(map[string]Material)
	ids := make(map[int64]bool)
	for _, entry := range material {
		if entry.ID <= 0 || ids[entry.ID] {
			return nil, nil, fmt.Errorf("invalid certificate identity")
		}
		ids[entry.ID] = true
		if _, err := Validate(entry.Pair, nil); err != nil {
			return nil, nil, err
		}
		name := Name(entry.ID, entry.Pair)
		if _, exists := byName[name]; exists {
			return nil, nil, fmt.Errorf("duplicate certificate material")
		}
		byName[name] = entry
	}
	keep := make(map[string]bool)
	for n, file := range files {
		certPath := strings.ReplaceAll(file.Certificate, `\`, "/")
		keyPath := strings.ReplaceAll(file.Key, `\`, "/")
		name := filepath.Base(filepath.Dir(filepath.FromSlash(certPath)))
		if filepath.Base(filepath.FromSlash(certPath)) != "cert.pem" || filepath.Base(filepath.FromSlash(keyPath)) != "key.pem" || filepath.Dir(filepath.FromSlash(certPath)) != filepath.Dir(filepath.FromSlash(keyPath)) {
			return nil, nil, fmt.Errorf("invalid managed certificate paths")
		}
		if _, ok := byName[name]; !ok || keep[name] {
			return nil, nil, fmt.Errorf("certificate loader has no unique manifest entry")
		}
		keep[name] = true
		files[n].Certificate = filepath.Join(root, name, "cert.pem")
		files[n].Key = filepath.Join(root, name, "key.pem")
	}
	if len(keep) != len(byName) {
		return nil, nil, fmt.Errorf("unreferenced private material in certificate manifest")
	}
	for _, entry := range byName {
		if _, err := StageRuntime(root, entry.ID, entry.Pair); err != nil {
			return nil, nil, err
		}
	}
	if len(files) > 0 {
		var err error
		certificates["load_files"], err = json.Marshal(files)
		if err != nil {
			return nil, nil, err
		}
		tlsApp["certificates"], err = json.Marshal(certificates)
		if err != nil {
			return nil, nil, err
		}
		apps["tls"], err = json.Marshal(tlsApp)
		if err != nil {
			return nil, nil, err
		}
		config["apps"], err = json.Marshal(apps)
		if err != nil {
			return nil, nil, err
		}
	}
	result, err := json.Marshal(config)
	return result, keep, err
}

func decodeObject(raw json.RawMessage, dest *map[string]json.RawMessage) error {
	*dest = make(map[string]json.RawMessage)
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, dest); err != nil || *dest == nil {
		return fmt.Errorf("invalid Caddy configuration object")
	}
	return nil
}

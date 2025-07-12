package keys

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
)

type KeyConfig struct {
	KID        string `json:"kid"`
	PublicKey  string `json:"publicKey"`
	PrivateKey string `json:"privateKey"`
}

type KeyStore struct {
	PublicKeys  map[string]ed25519.PublicKey
	PrivateKeys map[string]ed25519.PrivateKey
}

// LoadKeysFromFile carrega as chaves de um arquivo JSON.
func LoadKeysFromFile(path string) (*KeyStore, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler arquivo de chaves: %w", err)
	}

	var config struct {
		Keys []KeyConfig `json:"keys"`
	}
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, fmt.Errorf("falha ao decodificar JSON de chaves: %w", err)
	}

	keyStore := &KeyStore{
		PublicKeys:  make(map[string]ed25519.PublicKey),
		PrivateKeys: make(map[string]ed25519.PrivateKey),
	}

	for _, keyConf := range config.Keys {
		pub, err := base64.StdEncoding.DecodeString(keyConf.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("falha ao decodificar chave pública para kid %s: %w", keyConf.KID, err)
		}
		priv, err := base64.StdEncoding.DecodeString(keyConf.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("falha ao decodificar chave privada para kid %s: %w", keyConf.KID, err)
		}
		keyStore.PublicKeys[keyConf.KID] = ed25519.PublicKey(pub)
		keyStore.PrivateKeys[keyConf.KID] = ed25519.PrivateKey(priv)
	}

	return keyStore, nil
}

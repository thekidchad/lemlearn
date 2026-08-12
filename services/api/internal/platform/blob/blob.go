// Package blob range les fichiers volumineux : tracés de signature, PDF
// scellés, exports de dossier.
package blob

import (
	"context"
	"fmt"
	"sync"
)

// Store est le contrat commun à S3 et à l'implémentation mémoire.
type Store interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
}

// ErrNotFound est renvoyée par Get quand la clé n'existe pas.
var ErrNotFound = fmt.Errorf("blob: introuvable")

// Memory est un magasin en mémoire, pour les tests et le développement local
// sans compte AWS.
//
// Il ne simule pas Object Lock : ce qu'il permet — écraser une clé existante —
// est précisément ce que la production interdit. Il ne doit donc jamais servir
// à valider une règle de conservation.
type Memory struct {
	mu    sync.RWMutex
	items map[string][]byte
}

// NewMemory construit un magasin mémoire.
func NewMemory() *Memory {
	return &Memory{items: make(map[string][]byte)}
}

// Put range un contenu.
func (m *Memory) Put(_ context.Context, key, _ string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Copie défensive : l'appelant peut réutiliser son tampon, et un document
	// scellé qui changerait après coup dans le magasin serait indétectable.
	stored := make([]byte, len(data))
	copy(stored, data)
	m.items[key] = stored
	return nil
}

// Get relit un contenu.
func (m *Memory) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.items[key]
	if !ok {
		return nil, ErrNotFound
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
}

// Keys énumère les clés rangées. Réservé aux tests.
func (m *Memory) Keys() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]string, 0, len(m.items))
	for key := range m.items {
		keys = append(keys, key)
	}
	return keys
}

package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/Damirka228/PerkBack/services/rules-service/internal/config"
	storagepkg "github.com/Damirka228/PerkBack/services/rules-service/internal/storage"
)

func TestNewStorageInvalidDSN(t *testing.T) {
	storage, err := storagepkg.NewStorage(context.Background(), config.DB{
		DSN:            "postgres://%zz",
		ConnectTimeout: time.Second,
	})
	if err == nil || storage != nil {
		t.Fatalf("NewStorage() = (%v, %v), want nil storage and an error", storage, err)
	}
}

func TestStorageCloseWithoutPool(t *testing.T) {
	storage := &storagepkg.Storage{}
	storage.Close()
	storage.Close()
}

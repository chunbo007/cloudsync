package credentials

import "context"

type Store interface {
	Save(ctx context.Context, key string, value []byte) error
	Load(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}

type Cipher interface {
	Seal(key string, value []byte) ([]byte, error)
	Open(key string, value []byte) ([]byte, error)
}

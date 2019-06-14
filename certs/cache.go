package certs

import (
	"context"
	"errors"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/mholt/certmagic"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/clientv3/concurrency"
)

const (
	timeout    = 2 * time.Second
	modified   = "__Modified"
	lockPrefix = "/elinproxy/certs/mutex"
)

var (
	ErrCacheKeyNotFound = errors.New("Key not found")
)

func newStorage(endpoints []string) *Storage {
	if len(endpoints) == 0 {
		endpoints = append(endpoints, "http://localhost:2379")
	}
	etcdCli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: timeout,
	})
	if err != nil {
		return nil
	}

	return &Storage{
		cli: etcdCli,
		mu:  sync.Mutex{},
	}
}

type Storage struct {
	mumap sync.Map
	cli   *clientv3.Client
	mu    sync.Mutex
}

func (s *Storage) Store(key string, value []byte) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, err = s.cli.Put(ctx, key, string(value))
	if err != nil {
		return err
	}

	_, err = s.cli.Put(ctx, key+modified, strconv.Itoa(int(time.Now().Unix())))
	if err != nil {
		return err
	}

	return err
}

func (s *Storage) Load(key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	resp, err := s.cli.Get(ctx, key)
	cancel()
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Count == 0 {
		return nil, ErrCacheKeyNotFound
	}
	return resp.Kvs[0].Value, nil
}

func (s *Storage) Delete(key string) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, err = s.cli.Delete(ctx, key)
	if err != nil {
		return err
	}
	_, err = s.cli.Delete(ctx, key+modified)
	if err != nil {
		return err
	}

	return err
}

func (s *Storage) Exists(key string) bool {
	k, err := s.cli.Get(context.Background(), key)
	if err != nil || k == nil || k.Count == 0 {
		return false
	}
	return true
}

func (s *Storage) List(prefix string, recursive bool) ([]string, error) {
	resp, err := s.cli.Get(context.Background(), prefix, clientv3.WithPrefix())
	if err != nil || resp.Count == 0 {
		return nil, err
	}
	values := make([]string, 0)
	for _, v := range resp.Kvs {
		values = append(values, string(v.Value))
	}
	return nil, nil
}

// Stat returns information about key.
func (s *Storage) Stat(key string) (certmagic.KeyInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resp, err := s.cli.Get(ctx, key)
	if err != nil {
		return certmagic.KeyInfo{}, err
	}
	if resp == nil || resp.Count == 0 {
		return certmagic.KeyInfo{}, ErrCacheKeyNotFound
	}
	size := resp.Kvs[0].Size()

	resp, err = s.cli.Get(ctx, key+modified)
	if err != nil {
		return certmagic.KeyInfo{}, err
	}
	if resp == nil || resp.Count == 0 {
		return certmagic.KeyInfo{}, ErrCacheKeyNotFound
	}

	mod, err := strconv.Atoi(string(resp.Kvs[0].Value))
	if err != nil {
		return certmagic.KeyInfo{}, err
	}

	return certmagic.KeyInfo{
		Key:        key,
		Modified:   time.Unix(int64(mod), 0),
		IsTerminal: true,
		Size:       int64(size),
	}, nil
}

// Lock the underline etcd session key based
func (s *Storage) getLockerSession(key string) *concurrency.Mutex {
	sess, err := concurrency.NewSession(s.cli)
	if err != nil {
		log.Fatal(err)
	}
	localMu := concurrency.NewMutex(sess, lockPrefix+key)

	actual, loaded := s.mumap.LoadOrStore(key, localMu)
	if loaded {
		// We don't need the created session
		sess.Done()
	}
	return actual.(*concurrency.Mutex)
}

// Lock the underline etcd session key based
func (s *Storage) Lock(key string) (err error) {
	s.mu.Lock()
	localMu := s.getLockerSession(key)
	err = localMu.Lock(s.cli.Ctx())
	if err != nil {
		s.mu.Unlock()
	}
	return err
}

// Unlock the underline etcd session key based
func (s *Storage) Unlock(key string) (err error) {
	localMu := s.getLockerSession(key)
	err = localMu.Unlock(s.cli.Ctx())
	s.mu.Unlock()
	return err
}

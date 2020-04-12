package certs

import (
	"fmt"
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"go.etcd.io/etcd/clientv3"
	pb "go.etcd.io/etcd/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/integration"
	"go.etcd.io/etcd/proxy/grpcproxy"
	"google.golang.org/grpc"
)

func createLocalEtcdServer(t *testing.T) *integration.ClusterV3 {
	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	time.Sleep(100 * time.Millisecond)
	return clus
}

func TestCacheOneNodeLockUnlock(t *testing.T) {
	clus := createLocalEtcdServer(t)
	defer clus.Terminate(t)

	log.Printf("%+v", clus)

	c := newStorage([]string{clus.Members[0].GRPCAddr()})
	wg := sync.WaitGroup{}
	for i := 1; i <= 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tryLock(t, c, i)
		}(i)
	}
	wg.Wait()
}

func TestCacheMultiNodeLockUnlock(t *testing.T) {
	clus := createLocalEtcdServer(t)
	defer clus.Terminate(t)

	log.Printf("%+v", clus)

	wg := sync.WaitGroup{}
	for i := 1; i <= 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := newStorage([]string{clus.Members[0].GRPCAddr()})
			tryLock(t, c, i)
		}(i)
	}
	wg.Wait()
}

func tryLock(t *testing.T, c *Storage, n int) {
	lockName := "/lockName"
	if err := c.Lock(lockName); err != nil {
		t.Fatalf("Lock %s", err)
		return
	}

	key := fmt.Sprintf("commonKey")

	for i := 1; i <= 5; i++ {
		body := []byte(fmt.Sprintf("body%d%d", n, i))
		//t.Logf("I(%d): %s", n, key)
		c.Store(key, body)
	}

	b, err := c.Load(key)
	if err != nil {
		t.Fatalf("Load: %s", err)
	}

	expectedBody := fmt.Sprintf("body%d%d", n, 5)
	if string(b) != expectedBody {
		t.Errorf("Mutex don't work: %s != %s", string(b), expectedBody)
		return
	}
	t.Logf("OK %d | Body: %s", n, string(b))

	if err := c.Unlock(lockName); err != nil {
		t.Fatalf("Lock %s", err)
		return
	}
}

type kvproxyTestServer struct {
	kp     pb.KVServer
	c      *clientv3.Client
	server *grpc.Server
	l      net.Listener
}

func (kts *kvproxyTestServer) close() {
	kts.server.Stop()
	kts.l.Close()
	kts.c.Close()
}

func newKVProxyServer(endpoints []string, t *testing.T) *kvproxyTestServer {
	cfg := clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	}
	client, err := clientv3.New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	kvp, _ := grpcproxy.NewKvProxy(client)

	kvts := &kvproxyTestServer{
		kp: kvp,
		c:  client,
	}

	var opts []grpc.ServerOption
	kvts.server = grpc.NewServer(opts...)
	pb.RegisterKVServer(kvts.server, kvts.kp)

	kvts.l, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	go kvts.server.Serve(kvts.l)

	return kvts
}

package v1

import (
	"context"
	"fmt"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/f41gh7/vcd-csi/conf"
	"github.com/f41gh7/vcd-csi/internal/driver"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync"
)

type CsiController struct {
	l   *logrus.Entry
	c   *conf.ControllerConfig
	drv *driver.CsiDriver
	wg  *sync.WaitGroup
}

func NewCsiController(l *logrus.Entry, c *conf.ControllerConfig, drv *driver.CsiDriver, wg *sync.WaitGroup) (*CsiController, error) {

	return &CsiController{
		l:   l,
		c:   c,
		drv: drv,
		wg:  wg,
	}, nil
}

func (cs *CsiController) Run(ctx context.Context) error {
	defer cs.wg.Done()
	u, err := url.Parse(cs.c.UnixSocket)
	if err != nil {
		return fmt.Errorf("bad unix socket: %s", err)
	}

	grpcAddr := path.Join(u.Host, filepath.FromSlash(u.Path))
	if u.Host == "" {
		grpcAddr = filepath.FromSlash(u.Path)
	}

	if err := os.Remove(grpcAddr); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove unix domain socket file %s, error: %s", grpcAddr, err)
	}

	grpcListener, err := net.Listen(u.Scheme, grpcAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	// log response errors for better observability
	errHandler := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			cs.l.WithError(err).WithField("method", info.FullMethod).Error("method failed")
		}
		return resp, err
	}

	srv := grpc.NewServer(grpc.UnaryInterceptor(errHandler))
	csi.RegisterIdentityServer(srv, cs.drv)
	csi.RegisterControllerServer(srv, cs.drv)
	csi.RegisterNodeServer(srv, cs.drv)

	httpListener, err := net.Listen("tcp", cs.c.HttpListen)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		//err := d.healthChecker.Check(r.Context())
		//if err != nil {
		//	w.WriteHeader(http.StatusInternalServerError)
		//	return
		//}
		w.WriteHeader(http.StatusOK)
	})
	httpSrv := http.Server{
		Handler: mux,
	}

	//d.ready = true // we're now ready to go!
	//d.log.WithFields(logrus.Fields{
	//	"grpc_addr": grpcAddr,
	//	"http_addr": d.address,
	//}).Info("starting server")

	var eg errgroup.Group
	eg.Go(func() error {
		<-ctx.Done()
		return httpSrv.Shutdown(context.Background())
	})
	eg.Go(func() error {
		go func() {
			<-ctx.Done()
			cs.l.Info("server stopped")
			//d.readyMu.Lock()
			//d.ready = false
			//readyMu.Unlock()
			srv.GracefulStop()
		}()
		return srv.Serve(grpcListener)
	})
	eg.Go(func() error {
		err := httpSrv.Serve(httpListener)
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	})

	return eg.Wait()
}

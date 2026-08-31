package app

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"marketplace/internal/config"
	"marketplace/internal/domain"
	"marketplace/internal/handler"
	"marketplace/internal/migrate"
	"marketplace/internal/provider"
	"marketplace/internal/queue"
	"marketplace/internal/repository"
	"marketplace/internal/service"
	"marketplace/web"

	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	Cfg    config.Config
	Pool   *pgxpool.Pool
	Server *http.Server
	Jobs   *queue.Delivery
	Orders *service.Orders
	Pay    *service.Payment
	Ship   *service.Delivery
	Rec    *service.Recovery
	ln     net.Listener
	stop   context.CancelFunc
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("db connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}
	if err := migrate.Up(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	db := repository.New(pool)
	products := repository.NewProductRepo(db)
	inventory := repository.NewInventoryRepo(db)
	orders := repository.NewOrderRepo(db)
	events := repository.NewPaymentRepo(db)
	ledger := repository.NewLedgerRepo(db)
	attempts := repository.NewDeliveryRepo(db)

	if err := service.NewSeeder(products, inventory).Run(ctx, cfg.CatalogSeedExtra); err != nil {
		pool.Close()
		return nil, fmt.Errorf("seed: %w", err)
	}

	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		pool.Close()
		return nil, err
	}
	tcpAddr := ln.Addr().(*net.TCPAddr)
	base := fmt.Sprintf("http://127.0.0.1:%d", tcpAddr.Port)

	jobs := queue.NewDelivery(256)
	hangFor := cfg.ProviderTimeout * 3
	stubA := provider.NewStub(domain.ProviderA, db, inventory, products, cfg.ProviderAErrorRate, cfg.ProviderATimeoutRate, hangFor)
	stubB := provider.NewStub(domain.ProviderB, db, inventory, products, cfg.ProviderBErrorRate, cfg.ProviderBTimeoutRate, hangFor)

	pay := service.NewPayment(db, orders, events, ledger, jobs)
	orderSvc := service.NewOrders(products, orders, pay)
	catalog := service.NewCatalog(products)
	clientA := provider.NewClient(domain.ProviderA, base+"/internal/providers/a/issue", cfg.ProviderTimeout)
	clientB := provider.NewClient(domain.ProviderB, base+"/internal/providers/b/issue", cfg.ProviderTimeout)
	ship := service.NewDelivery(db, orders, ledger, attempts, clientA, clientB, 3, 80*time.Millisecond)
	reconcile := service.NewReconcile(orders, ledger)
	recovery := service.NewRecovery(orders, pay, ship, cfg.StuckAfter)

	restock := func(sku string, codes []string) error {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, code := range codes {
			if err := inventory.InsertKey(c, sku, code); err != nil {
				return err
			}
		}
		return products.SyncStock(c, sku)
	}

	api := handler.New(catalog, orderSvc, pay, ship, reconcile, recovery, stubA, stubB, restock)
	mux := http.NewServeMux()
	api.Routes(mux)
	mux.Handle("/", staticHandler())

	srv := &http.Server{
		Addr:              ln.Addr().String(),
		Handler:           withLog(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &App{
		Cfg:    cfg,
		Pool:   pool,
		Server: srv,
		Jobs:   jobs,
		Orders: orderSvc,
		Pay:    pay,
		Ship:   ship,
		Rec:    recovery,
		ln:     ln,
	}, nil
}

func (a *App) Serve() error {
	return a.Server.Serve(a.ln)
}

func (a *App) Start(ctx context.Context) {
	wctx, cancel := context.WithCancel(ctx)
	a.stop = cancel
	go func() {
		if err := a.Serve(); err != nil && err != http.ErrServerClosed {
			slog.Error("http_serve", "err", err)
		}
	}()
	a.StartWorkers(wctx)
}

func (a *App) StartWorkers(ctx context.Context) {
	go a.consume(ctx)
	go a.recoverLoop(ctx)
}

func (a *App) consume(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-a.Jobs.Jobs():
			jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			if err := a.Ship.Deliver(jobCtx, id); err != nil && err != domain.ErrInvalid {
				slog.Error("deliver_job", "order_id", id, "err", err)
			}
			cancel()
		}
	}
}

func (a *App) recoverLoop(ctx context.Context) {
	t := time.NewTicker(a.Cfg.WorkerInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.Rec.Tick(ctx)
		}
	}
}

func (a *App) Close(ctx context.Context) {
	if a.stop != nil {
		a.stop()
	}
	_ = a.Server.Shutdown(ctx)
	a.Pool.Close()
}

func (a *App) URL() string {
	tcpAddr := a.ln.Addr().(*net.TCPAddr)
	return fmt.Sprintf("http://127.0.0.1:%d", tcpAddr.Port)
}

func staticHandler() http.Handler {
	sub, err := fs.Sub(web.FS, ".")
	if err != nil {
		return http.FileServer(http.Dir("web"))
	}
	return http.FileServer(http.FS(sub))
}

func withLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/webhook") || strings.HasPrefix(r.URL.Path, "/admin") {
			slog.Info("http", "method", r.Method, "path", r.URL.Path, "dur_ms", time.Since(start).Milliseconds())
		}
	})
}

func InitLogger() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
}

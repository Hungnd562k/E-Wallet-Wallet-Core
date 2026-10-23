# Luồng `run()` của wallet-worker (pseudo code)

Nguồn: `Hungnd562k/E-Wallet-Wallet-Core`, nhánh `development` @ `eac0738` (số dòng trong file này theo nhánh này).

So với `main` (@ `85e987b`, đi sau `development`): `cmd/worker/main.go`, `internal/platform/{config,db,outbox}` giống hệt nhau. Khác duy nhất là `internal/app/app.go` trên `main` chưa có module `identity`, nên `NewModules` không tạo `identity.Service` (số dòng trong app.go lệch vài dòng). Luồng `run()` không đổi.

Ghi chú: hiện tại worker **chỉ chạy một thành phần là outbox Dispatcher**. Bank runner, merchant callback và scheduler chưa có, mới chỉ là comment chỗ sẽ thêm (`cmd/worker/main.go:54-56`). "Lease" và "retry/backoff" không phải goroutine riêng mà nằm bên trong Dispatcher.

## 1. `main()` (cmd/worker/main.go:23-28)

```text
main():
    err = run()
    nếu err != nil:
        in "worker: <err>" ra stderr
        os.Exit(1)            // exit code 1 để Docker/k8s biết process lỗi
    // run() trả nil → thoát bình thường, exit 0
```

`run()` trả `error` thay vì tự `os.Exit` để các `defer` bên trong (đóng pool, stop signal) chắc chắn chạy. Giống C#: `Main` gọi một hàm, hàm đó dùng `using`, còn `Main` mới quyết định exit code.

## 2. `run()` từng bước

```text
run() error:
    // ── B1. Đọc cấu hình từ env ─────────────────────── main.go:31-34
    cfg, err = config.Load()
    nếu err: return err                     // → main in lỗi, exit 1

    // ── B2. Logger mặc định ──────────────────────────── main.go:35
    slog.SetDefault( NewLogger(stdout, cfg.LogLevel) + field worker_id=cfg.WorkerID )
    // logger có redact các key nhạy cảm (pin, otp, token, ...)

    // ── B3. ctx bị hủy khi nhận Ctrl+C / SIGTERM ─────── main.go:37-38
    ctx, stop = signal.NotifyContext(Background, SIGINT, SIGTERM)
    defer stop()                            // gỡ handler tín hiệu khi run() kết thúc

    // ── B4. Kết nối Postgres ─────────────────────────── main.go:40-44
    pool, err = db.NewPool(ctx, cfg.DatabaseURL)
    nếu err: return err                     // DB không lên → exit 1 ngay
    defer pool.Close()                      // chạy SAU CÙNG khi run() return

    // ── B5. Lắp ráp thành phần (composition root) ─────── main.go:46-52
    clk = clock.Real{}
    modules = app.NewModules(pool, clk)     // ledger, wallet, identity (chỉ development), payment, notification
    dispatcher = app.NewDispatcher(pool, clk, modules, DispatcherConfig{
        WorkerID, BatchSize, PollInterval, MaxAttempts lấy từ cfg
        // Lease, Concurrency không set → default trong outbox.NewDispatcher:
        //   Lease = 1 phút, Concurrency = 4
    })
    // NewDispatcher đăng ký consumer:
    //   "payment.succeeded.v1" → notification.HandlePaymentSucceeded

    // ── B6. Chạy các goroutine dài hạn ─────────────────── main.go:57-58
    g, ctx = errgroup.WithContext(ctx)      // ctx mới: hủy khi có SIGTERM HOẶC 1 goroutine trả lỗi
    g.Go( dispatcher.Run(ctx) )             // goroutine #1 (duy nhất hiện tại)
    // tương lai: g.Go(bankRunner.Run(ctx)), g.Go(scheduler.Run(ctx))

    // ── B7. Chờ đến khi tắt ──────────────────────────── main.go:60-63
    log "wallet-worker started"
    err = g.Wait()                          // BLOCK ở đây cho tới khi mọi goroutine return
    log "wallet-worker stopped"
    return err
    // sau return: defer pool.Close(), rồi defer stop()   (LIFO)
```

### B1 chi tiết: `config.Load()` (internal/platform/config/config.go:38-71)

```text
Load():
    Env         = APP_ENV          (mặc định "local")
    DatabaseURL = DATABASE_URL     (bắt buộc)
    WorkerID    = WORKER_ID        (mặc định hostname)
    LogLevel    = LOG_LEVEL        (mặc định "info")
    OutboxBatchSize    = OUTBOX_BATCH_SIZE    (mặc định 20)
    OutboxMaxAttempts  = OUTBOX_MAX_ATTEMPTS  (mặc định 10)
    OutboxPollInterval = OUTBOX_POLL_INTERVAL (mặc định 1s)
    gom TẤT CẢ lỗi parse / thiếu biến vào errs  (không dừng ở lỗi đầu tiên)
    APP_ENV phải là "local" hoặc "test"
    return cfg, errors.Join(errs...)    // báo hết lỗi cấu hình một lần
```

### B4 chi tiết: `db.NewPool()` (internal/platform/db/db.go:26-36)

```text
NewPool(ctx, url):
    pool, err = pgxpool.New(ctx, url)   // chỉ parse URL, chưa chắc đã kết nối
    nếu err: return "create pool: ..."
    nếu pool.Ping(ctx) lỗi:
        pool.Close()
        return "ping database: ..."     // fail-fast: DB chết thì worker không khởi động
    return pool
```

## 3. Bên trong goroutine: `Dispatcher.Run` (internal/platform/outbox/dispatcher.go:67-85)

```text
Run(ctx):
    ticker = mỗi PollInterval (1s)
    lặp mãi:
        n, err = RunOnce(ctx)
        nếu err và ctx CHƯA bị hủy: log "outbox poll failed"   // lỗi poll KHÔNG làm chết worker
        nếu n > 0 và không lỗi: continue      // còn việc → poll tiếp ngay, không chờ tick
        select:
            ctx.Done()  → return nil          // ← đường tắt máy, luôn trả nil
            ticker.C    → vòng lặp tiếp
```

### `RunOnce` (dispatcher.go:89-112): claim + lease

```text
RunOnce(ctx):
    now = clock.Now()
    events = ClaimEvents(worker_id, lease_until = now + Lease, now, batch)
        // SQL (outbox/queries.sql:6-19):
        //   UPDATE outbox_events SET status='PROCESSING', locked_by, locked_until
        //   WHERE id IN (SELECT ... status IN (PENDING, PROCESSING)
        //                 AND available_at <= now
        //                 AND (locked_until IS NULL OR locked_until < now)   ← lease hết hạn thì worker khác lấy lại
        //                 ORDER BY available_at LIMIT batch
        //                 FOR UPDATE SKIP LOCKED)                          ← nhiều worker không chặn nhau
    nếu err: return 0, "claim events: ..."

    workCtx = context.WithoutCancel(ctx)   // tách khỏi SIGTERM: batch đang chạy được làm cho xong
    g = errgroup với SetLimit(Concurrency = 4)
    với mỗi event: g.Go( process(event); return nil )   // lỗi từng event không hủy cả batch
    return len(events), g.Wait()
```

### `process` (dispatcher.go:114-141): retry / dead-letter

```text
process(ctx, e):
    msg = Message từ row; gắn request_id từ headers vào ctx
    err = deliver(ctx, msg)
    switch:
        err == nil                       → MarkDone      (status DONE, attempts+1)
        e.Attempts+1 >= MaxAttempts (10) → MarkDead      (status DEAD, lưu last_error)
        còn lại                          → delay = Backoff(attempts+1)
                                           MarkRetry     (status PENDING, available_at = now + delay)
    // lỗi khi Mark* chỉ log; lease hết hạn thì event được claim lại
```

`Backoff` (outbox/backoff.go:16-33): 1s, 2s, 4s, ... tối đa 5 phút, nhân jitter ±20% để các event lỗi không retry cùng lúc.

### `deliver` (dispatcher.go:146-165): at-least-once nhưng hiệu quả là once

```text
deliver(ctx, msg):
    với mỗi consumer đăng ký cho msg.Type:
        db.WithTx:                                  // mỗi consumer 1 transaction riêng
            n = INSERT processed_events(consumer, event_id) ON CONFLICT DO NOTHING
            nếu n == 0: return nil                  // consumer này đã xử lý rồi → bỏ qua
            return consumer.handler(ctx, msg)       // handler lỗi → rollback cả marker
        nếu err: return "consumer <name>: ..."      // dừng, cả event sẽ retry
    return nil
```

## 4. Graceful shutdown (khi nhận SIGTERM / Ctrl+C)

```text
1. signal.NotifyContext hủy ctx gốc  → ctx của errgroup cũng bị hủy
2. Dispatcher.Run:
     - nếu đang chờ tick: select thấy ctx.Done() → return nil ngay
     - nếu đang trong RunOnce: batch hiện tại vẫn chạy hết (workCtx WithoutCancel),
       rồi quay lại select → return nil
     - event nào chưa xong (vd process bị kill cứng) → lease 1 phút hết hạn → worker khác claim lại
3. g.Wait() trả nil → log "wallet-worker stopped" → run() return nil
4. defer pool.Close() → defer stop() → main thấy nil → exit 0
```

## 5. Bảng các đường lỗi

| Tình huống | Chỗ | Kết quả |
|---|---|---|
| Thiếu `DATABASE_URL`, env sai định dạng, `APP_ENV` lạ | main.go:31-34 | `run` trả lỗi (đã gộp mọi lỗi), exit 1 |
| Không kết nối / ping được DB lúc khởi động | main.go:40-43, db.go:27-34 | exit 1, pool đã được đóng |
| DB chập chờn khi đang chạy (claim lỗi) | dispatcher.go:73-75 | chỉ log, thử lại ở tick sau; worker **không** chết |
| Consumer lỗi | dispatcher.go:132-139 | MarkRetry với backoff |
| Consumer lỗi lần thứ 10 | dispatcher.go:127-131 | MarkDead, cần người xử lý |
| Mark* lỗi | dispatcher.go:124-138 | chỉ log; lease hết hạn thì xử lý lại (consumer đã xong được bỏ qua nhờ `processed_events`) |
| Một goroutine trong errgroup trả lỗi | main.go:57-61 | errgroup hủy ctx → các goroutine khác dừng → `g.Wait` trả lỗi đầu tiên → exit 1. Hiện tại `Dispatcher.Run` luôn trả `nil` nên đường này chưa xảy ra; nó dành cho bank runner/scheduler sau này |

## 6. Hai điểm đáng chú ý

1. `Lease` và `Concurrency` không có biến môi trường; chúng luôn là mặc định 1 phút và 4 (`dispatcher.go:48-53`). Nếu handler nào chạy lâu hơn 1 phút, worker khác có thể claim lại event đó. Marker `processed_events` vẫn chặn xử lý trùng nhờ transaction, nhưng nên nhớ khi thêm consumer chậm.
2. Biến `ctx` ở `main.go:57` bị **gán đè** (`g, ctx := errgroup.WithContext(ctx)`), nên từ dòng đó trở đi `ctx` là ctx của errgroup chứ không còn là ctx tín hiệu. Đây là pattern Go chuẩn, nhưng người từ C# dễ đọc nhầm.

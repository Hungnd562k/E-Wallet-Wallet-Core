# 06 — Concurrency

Bài tập: [`learn/06_concurrency`](../../learn/06_concurrency/). **Luôn chạy với `-race`**: `go test -race ./learn/06_concurrency -v`.

## So sánh nhanh

| C# | Go |
|---|---|
| `Task.Run(() => ...)` | `go f()`: goroutine, rẻ (vài KB stack), có thể chạy hàng trăm nghìn |
| `async/await` | **Không có.** Code viết đồng bộ; khi I/O block, runtime tự chuyển sang goroutine khác |
| `await Task.WhenAll(...)` | `sync.WaitGroup` hoặc `errgroup.Group` |
| `CancellationToken` | `context.Context`, luôn là **tham số đầu tiên** |
| `CancellationTokenSource.CancelAfter` | `context.WithTimeout(ctx, d)` + `defer cancel()` |
| `lock (obj) { }` | `mu.Lock(); defer mu.Unlock()` |
| `Interlocked.Increment` | `atomic.Int64.Add(1)` |
| `Channel<T>` / `BlockingCollection` | `chan T`, có sẵn trong ngôn ngữ |
| `Task.WhenAny` | `select { case <-a: ... case <-b: ... }` |
| `AsyncLocal<T>` | Giá trị trong `context.Context` (`context.WithValue`) |

**Không có `async` lan truyền.** Hàm `Transfer` vừa gọi DB (I/O) vừa là một hàm bình thường. Không cần `Task<T>` hay `ConfigureAwait`.

## Hai loại "đồng thời" trong repo

1. **Trong tiến trình**: goroutine, channel, mutex, errgroup.
2. **Giữa các request hoặc process**: dùng **lock của database**. Hai request chuyển tiền chạy trên hai goroutine (có thể ở hai instance khác nhau), nên mutex trong Go **không** bảo vệ được. Repo dựa vào `SELECT … FOR UPDATE` theo thứ tự id tăng dần và `FOR UPDATE SKIP LOCKED` cho outbox.

## Thấy ở đâu trong repo

- **Mỗi request HTTP chạy trên một goroutine riêng**: `net/http` tự làm việc này. Vì vậy mọi service phải an toàn khi bị gọi đồng thời. Service của repo không giữ state mutable nào, ngoài pool (vốn đã thread-safe).
- **Graceful shutdown**: `signal.NotifyContext` ([`cmd/api/main.go:40`](../../cmd/api/main.go#L40)) tạo một ctx bị huỷ khi nhận SIGTERM. Server chạy trong goroutine ([dòng 60](../../cmd/api/main.go#L60)), và `select` ([dòng 65](../../cmd/api/main.go#L65)) chờ một trong hai: server lỗi, hoặc có tín hiệu tắt.
- **errgroup quản lý các thành phần chạy lâu**: [`cmd/worker/main.go:57`](../../cmd/worker/main.go#L57).
- **Vòng lặp poll + `select` với ticker**: `Dispatcher.Run` ở [`internal/platform/outbox/dispatcher.go:67`](../../internal/platform/outbox/dispatcher.go#L67) và [`case <-ctx.Done()`](../../internal/platform/outbox/dispatcher.go#L80).
- **Worker pool có giới hạn**: `errgroup.WithContext` + `g.SetLimit` ([`dispatcher.go:103`](../../internal/platform/outbox/dispatcher.go#L103)).
- **Làm nốt việc dở khi đang tắt**: `context.WithoutCancel(ctx)` ([`dispatcher.go:102`](../../internal/platform/outbox/dispatcher.go#L102)). Batch đang xử lý vẫn chạy xong, nhưng không nhận batch mới.
- **Mutex bảo vệ state dùng chung**: `clock.Fake` ([`internal/platform/clock/clock.go:25`](../../internal/platform/clock/clock.go#L25)).
- **Lock DB theo thứ tự để tránh deadlock**: `LockAccounts ... ORDER BY id FOR UPDATE` ([`internal/ledger/service.go:128`](../../internal/ledger/service.go#L128)) và `LockActive` ([`internal/wallet/service.go:145`](../../internal/wallet/service.go#L145)).
- **Idempotency khi nhiều request đến đồng thời**: `INSERT … ON CONFLICT DO NOTHING` sẽ **chờ** transaction đang giữ cùng key ([`internal/platform/idempotency/idempotency.go:84`](../../internal/platform/idempotency/idempotency.go#L84)).
- **Test đồng thời thật**: [`test/integration/transfer_test.go:113`](../../test/integration/transfer_test.go#L113) cho 50 goroutine chuyển chéo A↔B, và [`dòng 159`](../../test/integration/transfer_test.go#L159) cho 10 request cùng một key. Chú ý `tryTransfer` ([`helpers_test.go:107`](../../test/integration/helpers_test.go#L107)): **không được gọi `t.Fatal` từ goroutine khác goroutine của test**, thay vào đó dùng `t.Error` rồi `return`.

## Các bẫy thường gặp

- **Data race**: hai goroutine cùng ghi một biến mà không khóa. Nhớ chạy `-race`.
- **Goroutine leak**: goroutine chờ channel mãi không ai gửi. Khắc phục bằng channel có buffer, hoặc `select` thêm nhánh `ctx.Done()` (xem `CallWithTimeout` trong `learn`).
- **Đóng channel**: chỉ **bên gửi** đóng channel. Gửi vào channel đã đóng sẽ panic.
- **Copy mutex**: struct chứa `sync.Mutex` phải được dùng qua con trỏ. `go vet` sẽ cảnh báo.
- **Map không thread-safe**: ghi map đồng thời sẽ crash. Dùng mutex, hoặc `sync.Map` cho vài trường hợp đặc biệt.

## Bài tập

1. Viết `RateLimiter` trong [`learn/06_concurrency/concurrency.go`](../../learn/06_concurrency/concurrency.go) (token bucket bằng channel có buffer).
2. Xóa `mu.Lock()/Unlock()` trong `SafeCounter.Inc`, chạy `go test -race ./learn/06_concurrency` và đọc báo cáo race.
3. Trong `LockAccounts` ([`internal/ledger/store/queries.sql`](../../internal/ledger/store/queries.sql)), tạm bỏ `ORDER BY id`, chạy `make sqlc` rồi `make test-integration RUN=TestConcurrentCrossTransfers` vài lần. Postgres không đảm bảo thứ tự khóa khi không có `ORDER BY`, nên có thể (không chắc lần nào cũng) gặp lỗi `deadlock detected`. Vì sao thứ tự khóa cố định lại ngăn được deadlock? (Nhớ hoàn tác.)

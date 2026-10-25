# 09 — Lần theo một request `POST /v1/transfers`

Chương này nối mọi khái niệm trước lại với nhau. Mở từng link theo thứ tự, hoặc đặt breakpoint (VS Code + Go extension, hoặc `dlv debug ./cmd/api`) rồi gọi `curl` như ở [00-index](00-index.md).

## 0. Khởi động

[`cmd/api/main.go:32`](../../cmd/api/main.go#L32) chạy `run()` theo thứ tự: `config.Load`, tạo logger JSON có redaction, tạo pool Postgres, rồi `app.NewModules` ([`internal/app/app.go:38`](../../internal/app/app.go#L38)) dựng toàn bộ service. `app.NewAPIHandler` ([`app.go:50`](../../internal/app/app.go#L50)) gắn middleware và route.

## 1. Middleware (theo thứ tự)

1. `httpx.RequestID`: nhận hoặc sinh `X-Request-Id`, lưu vào `ctx`.
2. `httpx.Recover` ([`middleware.go:30`](../../internal/platform/httpx/middleware.go#L30)): panic biến thành 500.
3. `httpx.AccessLog`: mỗi request ghi một dòng log.
4. `auth.Middleware(DevHeaderAuth{})` ([`auth.go:32`](../../internal/platform/auth/auth.go#L32)): đọc `X-Dev-Customer-Id`, đặt `auth.Customer` vào ctx. Không hợp lệ thì trả 401.

## 2. Handler (transport)

[`internal/payment/http/handler.go:49`](../../internal/payment/http/handler.go#L49):

- Đọc body (giới hạn 1 MiB), decode **strict** (từ chối field lạ) vào `transferRequest` ([dòng 38](../../internal/payment/http/handler.go#L38)).
- Parse `wal_…` thành UUID (`id.Parse`).
- Gọi `payment.Service.Transfer` với một `TransferCommand`. Handler **không** chứa luật nghiệp vụ.
- Thành công thì trả `201` cùng JSON của `payment.Payment`; lỗi thì gọi `httpx.WriteError`.

## 3. Service: một transaction, mọi thứ cùng commit hoặc cùng rollback

[`internal/payment/service.go:104`](../../internal/payment/service.go#L104):

1. Tính `hash` của payload, dùng để phát hiện cùng key nhưng khác nội dung.
2. `db.WithTx` ([`db.go:44`](../../internal/platform/db/db.go#L44)) mở transaction và **đặt `pgx.Tx` vào ctx**. Từ đây, mọi module nhận ctx này đều ghi vào cùng transaction, vì repository lấy connection qua `db.Conn(ctx, pool)` ([`db.go:74`](../../internal/platform/db/db.go#L74)).
3. `idem.Begin` ([dòng 127](../../internal/payment/service.go#L127)) có ba khả năng:
   - Key mới thì đi tiếp.
   - Key đã hoàn tất thì `replay`, trả kết quả cũ.
   - Hash khác thì trả `422 IDEMPOTENCY_KEY_REUSED`.
4. `transferInTx` ([dòng 162](../../internal/payment/service.go#L162)):
   - Tìm ví nguồn của customer, rồi `domain.Transfer.Validate()`. Đây là luật thuần, không cần DB.
   - *(Chỗ TODO cho `authz` và `limit`: [dòng 178](../../internal/payment/service.go#L178).)*
   - `wallets.LockActive` ([dòng 181](../../internal/payment/service.go#L181)): `SELECT … FOR UPDATE` cả hai ví theo thứ tự id và kiểm tra cả hai đều `ACTIVE`.
   - `ledger.Post` ([dòng 188](../../internal/payment/service.go#L188) → [`ledger/service.go:118`](../../internal/ledger/service.go#L118)):
     - validate D = C;
     - lock các account theo thứ tự id;
     - insert transaction và entries;
     - số dư cuối âm thì trả `ErrInsufficientFunds`;
     - update balance.
   - Insert `payment` (`SUCCEEDED`), `status_history`, và 2 dòng `wallet_activities` (read-model).
   - `outbox.Enqueue` ([dòng 262](../../internal/payment/service.go#L262)): ghi event `payment.succeeded.v1` **trong cùng transaction**.
5. `idem.Complete` lưu response. Callback trả `nil`, nên `WithTx` commit.
6. Nếu có lỗi nghiệp vụ (ví dụ thiếu tiền), transaction rollback. Sau đó `SaveFailure` lưu lỗi vào idempotency key, nằm **ngoài** transaction đã rollback ([dòng 151](../../internal/payment/service.go#L151)), để lần retry nhận đúng lỗi đó.

Ba lớp phòng thủ cho "số dư không âm":

- app kiểm tra số dư cuối trong `ledger.Post`;
- `CHECK (allow_negative OR balance >= 0)` ở DB;
- deferred trigger kiểm tra Σdebit = Σcredit lúc `COMMIT`.

## 4. Worker: side-effect bất đồng bộ

[`internal/platform/outbox/dispatcher.go:89`](../../internal/platform/outbox/dispatcher.go#L89), `RunOnce`:

1. `ClaimEvents` dùng `UPDATE … WHERE id IN (SELECT … FOR UPDATE SKIP LOCKED)` để lease một batch event. Các worker khác bỏ qua những dòng đang bị khóa.
2. Mỗi event chạy trong một goroutine (errgroup có giới hạn). `deliver` ([dòng 146](../../internal/platform/outbox/dispatcher.go#L146)) với từng consumer:
   - mở transaction;
   - `INSERT processed_events` (đã có thì bỏ qua);
   - gọi handler.

   Kết quả là **at-least-once delivery + consumer idempotent**, tương đương xử lý đúng một lần.
3. Thành công thì `DONE`. Lỗi thì `PENDING` cùng `available_at = now + backoff`. Lỗi quá `MaxAttempts` thì `DEAD`.
4. Handler của `notification` ([`notification/service.go:37`](../../internal/notification/service.go#L37)) decode payload theo struct riêng của nó rồi log. Log có `request_id` của request HTTP gốc, vì header của event mang theo nó.

## 5. Kiểm chứng

- `make test-integration` chạy toàn bộ luồng trên với Postgres thật, có 50 goroutine đồng thời và `-race`.
- Truy vấn trực tiếp:

  ```bash
  docker compose exec postgres psql -U wallet -d wallet \
    -c "select code, balance from ledger.accounts order by code" \
    -c "select status, attempts, event_type from platform.outbox_events"
  ```

## Việc tiếp theo nên tự làm

1. Thêm `GET /v1/wallet` để trả ví và số dư của customer: handler mới trong `internal/wallet/http/`, route trong `app.go`.
2. Viết module `limit` (`CheckAndConsume`) và gọi nó ở chỗ TODO trong `transferInTx`. Thêm migration `0005_limits_init`, cập nhật `sqlc.yaml`, `make sqlc`.
3. Viết test cho mỗi bước trên trước khi code (TDD).

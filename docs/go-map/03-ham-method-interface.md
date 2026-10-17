# 03 — Hàm, method, interface và dependency injection

Bài tập: [`learn/03_interfaces`](../../learn/03_interfaces/).

## So sánh nhanh

| C# | Go |
|---|---|
| `class Sms : INotifier` | Không khai báo. `Sms` có method `Notify(...)` đúng chữ ký thì **tự động** thỏa `Notifier` |
| Interface đặt cạnh implementation | Interface đặt **ở phía sử dụng**, nhỏ, thường 1–3 method |
| Extension method | Method gắn vào bất kỳ kiểu nào **trong cùng package** |
| Kế thừa `class B : A` | **Embedding**: nhúng struct/interface, method được "promote". Đây là composition, không có `virtual`/`override` |
| `Func<int, int>` | `func(int) int`: hàm là giá trị bình thường |
| Lambda bắt biến (closure) | Giống hệt: closure giữ tham chiếu tới biến |
| `services.AddScoped<IFoo, Foo>()` | Gọi constructor bằng tay trong composition root |
| Moq | Fake viết tay (struct nhỏ), hiếm khi cần thư viện mock |

## Ba câu khẩu quyết

1. **"Accept interfaces, return structs."** Constructor nhận interface (để dễ thay khi test) và trả về con trỏ struct cụ thể.
2. **"The bigger the interface, the weaker the abstraction."** `io.Reader` chỉ có đúng 1 method.
3. **Interface do bên dùng định nghĩa.** `banklink` sẽ định nghĩa `BankGateway`; adapter corebank chỉ cần có đúng method là thỏa mãn (architecture §4.3 rule 5).

Kiểm tra lúc compile rằng một kiểu thỏa interface:

```go
var _ Notifier = (*SMS)(nil)
```

## DI không cần container

[`internal/app/app.go:38`](../../internal/app/app.go#L38) là toàn bộ "container" của repo:

```go
l := ledger.NewService(pool, clk)
w := wallet.NewService(pool, l)
p := payment.NewService(pool, clk, l, w, idempotency.NewStore(pool, clk), outbox.NewWriter(pool, clk))
```

Thứ tự khởi tạo hiện rõ ngay trong code, và quên một dependency là compiler báo lỗi. Thứ bạn đánh đổi là phải tự sửa danh sách tham số khi thêm dependency.

## Thấy ở đâu trong repo

- **Interface nhỏ ở phía dùng**: `DBTX` ở [`internal/platform/db/db.go:20`](../../internal/platform/db/db.go#L20). Cả `*pgxpool.Pool` lẫn `pgx.Tx` đều thỏa mà không cần khai báo gì.
- **Interface để thay được khi test**: `Clock` ở [`internal/platform/clock/clock.go:10`](../../internal/platform/clock/clock.go#L10), có `Real` và `Fake` ([dòng 24](../../internal/platform/clock/clock.go#L24)). Test outbox dùng `clock.NewFake` để "tua" thời gian ([`test/integration/outbox_test.go:60`](../../test/integration/outbox_test.go#L60)).
- **Interface strategy**: `Authenticator` ở [`internal/platform/auth/auth.go:25`](../../internal/platform/auth/auth.go#L25). Hiện tại chỉ có `DevHeaderAuth`; sau này thêm JWT thì chỉ cần đổi một dòng trong `cmd/api`.
- **Kiểu hàm làm interface**: `type Handler func(ctx, Message) error` ở [`internal/platform/outbox/dispatcher.go:22`](../../internal/platform/outbox/dispatcher.go#L22). Truyền **method value** `m.Notification.HandlePaymentSucceeded` vào `Register` ở [`internal/app/app.go:76`](../../internal/app/app.go#L76). Method value là method đã gắn sẵn receiver, giống delegate trong C#.
- **Middleware là hàm trả về hàm**: `func Middleware(a Authenticator) func(http.Handler) http.Handler` ở [`auth.go:32`](../../internal/platform/auth/auth.go#L32).
- **Embedding để decorate**: `statusWriter` nhúng `http.ResponseWriter` và chỉ ghi đè `WriteHeader`: [`internal/platform/httpx/middleware.go:59`](../../internal/platform/httpx/middleware.go#L59).
- **Hợp đồng phía consumer**: `notification` tự khai báo struct payload mà nó cần, thay vì import `payment`: [`internal/notification/service.go:27`](../../internal/notification/service.go#L27).

## Bài tập

1. Viết `Masker` trong [`learn/03_interfaces/interfaces.go`](../../learn/03_interfaces/interfaces.go).
2. Viết `type MaskingNotifier struct{ Notifier }`, nhúng một `Notifier` và che số điện thoại trước khi gọi `Notify` bên trong. Thêm dòng `var _ Notifier = (*MaskingNotifier)(nil)`.
3. Trong repo, tìm mọi chỗ gọi `NewService(`. Hãy vẽ lại đồ thị dependency và so với architecture §4.3.

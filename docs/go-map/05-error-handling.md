# 05 — Error handling

Bài tập: [`learn/05_errors`](../../learn/05_errors/).

## Triết lý

Go **không có exception** cho lỗi thông thường. Lỗi là một **giá trị** kiểu `error` (interface có duy nhất method `Error() string`), được trả về như kết quả thứ hai:

```go
p, err := svc.Transfer(ctx, cmd)
if err != nil {
    return fmt.Errorf("transfer for %s: %w", cmd.CustomerID, err) // thêm ngữ cảnh rồi trả lên
}
```

Mẫu `if err != nil` sẽ lặp lại rất nhiều. Đó là chủ ý: bạn nhìn thấy mọi đường lỗi ngay trong code.

| C# | Go |
|---|---|
| `throw new InsufficientFundsException()` | `return ErrInsufficientFunds` |
| `catch (InsufficientFundsException)` | `if errors.Is(err, ErrInsufficientFunds)` |
| `catch (LimitException e) { e.Limit }` | `var le *LimitError; if errors.As(err, &le) { le.Limit }` |
| `throw new X("ctx", inner)` | `fmt.Errorf("ctx: %w", err)` (`%w` = wrap, giữ lỗi gốc) |
| `AggregateException` | `errors.Join(errs...)` |
| Exception bất ngờ | `panic`, **chỉ** dành cho bug hoặc trạng thái không thể xảy ra |
| `try/finally` | `defer` |
| Global exception middleware | `recover()` trong middleware ([`httpx.Recover`](../../internal/platform/httpx/middleware.go#L30)) |

## Ba loại lỗi

1. **Sentinel**: biến cấp package, so bằng `errors.Is`. Ví dụ: `pgx.ErrNoRows`, `ledger.ErrInsufficientFunds`.
2. **Kiểu lỗi tùy biến**: struct có method `Error()`, lấy ra bằng `errors.As`. Ví dụ: `*pgconn.PgError`, `*apperr.Error`.
3. **Lỗi mờ** (opaque): `errors.New("...")` hoặc `fmt.Errorf` không bọc gì. Caller chỉ cần biết là có lỗi.

## Mô hình lỗi của repo

```text
domain/service  ──return──▶  *apperr.Error{Kind, Code, Message}   (không biết HTTP)
                    │ (có thể được bọc bằng %w qua nhiều tầng)
httpx.WriteError ──errors.As──▶ Kind → HTTP status, Code → {"error": "..."}
mọi lỗi khác     ──────────────▶ 500 INTERNAL_ERROR (log chi tiết, không lộ ra client)
```

- **`apperr.Error`** và `Kind` ở [`internal/platform/apperr/apperr.go:14`](../../internal/platform/apperr/apperr.go#L14). Kind phân loại theo **cách caller nên phản ứng**, không theo nơi phát sinh lỗi.
- **Custom `Is`** để bản sao do `WithDetails` tạo ra vẫn khớp sentinel: [`apperr.go:46`](../../internal/platform/apperr/apperr.go#L46).
- **Map sang HTTP** ở một chỗ duy nhất: [`internal/platform/httpx/httpx.go:41`](../../internal/platform/httpx/httpx.go#L41).
- **Sentinel nghiệp vụ**: `ErrInsufficientFunds` ở [`internal/ledger/domain/domain.go:66`](../../internal/ledger/domain/domain.go#L66).
- **Đổi lỗi hạ tầng thành lỗi nghiệp vụ**: `errors.Is(err, pgx.ErrNoRows)` thành `ErrAccountNotFound` ở [`internal/ledger/service.go:84`](../../internal/ledger/service.go#L84).
- **Đọc mã lỗi Postgres** bằng `errors.As(err, &pgErr)`: [`internal/platform/db/db.go:98`](../../internal/platform/db/db.go#L98).
- **Gom nhiều lỗi** khi validate config: `errors.Join` trong [`config.Load`](../../internal/platform/config/config.go#L38).
- **Lỗi làm rollback transaction**: callback trong `db.WithTx` trả `err != nil` thì toàn bộ bị rollback ([`db.go:44`](../../internal/platform/db/db.go#L44)). Đây là "exception làm rollback" kiểu Go.
- **Phân biệt lỗi nghiệp vụ và lỗi hạ tầng**: chỉ lỗi nghiệp vụ mới được lưu vào idempotency key để replay; lỗi hạ tầng được phép retry. Xem [`internal/payment/service.go:151`](../../internal/payment/service.go#L151).

## Quy ước

- Thông báo lỗi viết thường, không có dấu chấm cuối: `"insert entry: %w"`. Khi được bọc nhiều tầng, nó đọc thành một chuỗi liền: `transfer: insert entry: connection refused`.
- Mỗi tầng hoặc **xử lý** lỗi, hoặc **thêm ngữ cảnh rồi trả lên**. Không làm cả log lẫn return, vì sẽ log trùng.
- Không bao giờ `panic` cho lỗi nghiệp vụ. `uuid.Must`/`regexp.MustCompile` chỉ dùng khi lỗi nghĩa là bug.

## Bài tập

1. Viết `TransferAll` trong [`learn/05_errors/errors.go`](../../learn/05_errors/errors.go): bọc từng lỗi kèm index (`%w`) rồi `errors.Join`.
2. Trong `ledger.Post` ([`internal/ledger/service.go`](../../internal/ledger/service.go)), thử đổi `return uuid.Nil, ErrInsufficientFunds` thành `return uuid.Nil, errors.New("insufficient")` rồi chạy `make test-integration`. Test nào fail và HTTP status đổi thành gì? (Nhớ hoàn tác.)
3. Đọc `replay` ở [`internal/payment/service.go:301`](../../internal/payment/service.go#L301): lỗi được serialize thành JSON rồi dựng lại khi replay. Vì sao `Kind` cần có JSON tag?

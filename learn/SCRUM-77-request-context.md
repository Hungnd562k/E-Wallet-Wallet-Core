# SCRUM-77 · request_id, trace_id, ip, user agent đi từ HTTP tới audit như thế nào

> Ticket: <https://e-wallet-platform.atlassian.net/browse/SCRUM-77> (ST-3 của SCRUM-20).
> Ghi chú này giải thích code trong PR #7, so sánh với C#/ASP.NET Core.

## 1. Ý tưởng: dữ liệu của request nằm trong `context.Context`

Trong ASP.NET Core có `HttpContext`, và code ở tầng service lấy nó qua `IHttpContextAccessor`
(bên dưới là `AsyncLocal<T>`). Go không có "biến ngầm theo request" như vậy.
Thay vào đó, **mọi hàm nhận `ctx context.Context` làm tham số đầu tiên** và dữ liệu của request đi theo `ctx`.

```
HTTP request
  └─ httpx.RequestID   → ctx += request_id, trace_id
     └─ httpx.ClientInfo → ctx += Client{IP, UserAgent}
        └─ handler → payment.Transfer(ctx) → db.WithTx(ctx) → audit.Record(ctx, entry)
                                                              └─ đọc request_id, trace_id, IP, UA từ ctx
```

`context.WithValue(ctx, key, value)` trả về một ctx **mới** bọc ctx cũ, giống immutable dictionary.
Đọc lại bằng `ctx.Value(key).(KiểuGì)`. Dấu `.(T)` là type assertion, tương tự `as T` trong C#.

## 2. Vì sao key là `struct{}` riêng?

```go
type traceIDKey struct{}
ctx = context.WithValue(ctx, traceIDKey{}, id)
```

Nếu dùng key kiểu `string` như `"trace_id"`, package khác có thể vô tình dùng trùng và ghi đè giá trị.
Một kiểu chưa export (`traceIDKey`) chỉ package `observability` tạo ra được, nên không ai đụng vào được.
Ra bên ngoài chỉ có hàm `WithTraceID` / `TraceID`, giống việc bọc `HttpContext.Items` sau một service có kiểu rõ ràng.
File: `internal/platform/observability/context.go`.

## 3. Middleware trong Go

Middleware trong Go là một hàm `func(next http.Handler) http.Handler`,
tương đương `app.Use(async (ctx, next) => { ...; await next(); })` trong ASP.NET Core.

```go
func ClientInfo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := observability.WithClient(r.Context(), observability.Client{IP: ip, UserAgent: ua})
		next.ServeHTTP(w, r.WithContext(ctx)) // gọi tiếp với request mang ctx mới
	})
}
```

Thứ tự được đăng ký trong `internal/app/app.go`: `r.Use(httpx.RequestID, httpx.ClientInfo, httpx.Recover, httpx.AccessLog)`.

## 4. trace_id lấy từ đâu

- Chuẩn W3C Trace Context: header `traceparent: 00-<32 hex trace id>-<16 hex span id>-<2 hex flags>`.
- `httpx.RequestID` tách lấy phần trace id. Nếu header không có hoặc sai định dạng thì tự sinh 16 byte ngẫu nhiên (32 hex).
- Nhờ vậy cột `trace_id NOT NULL` luôn có giá trị thật. Khi thêm OpenTelemetry sau này, phần này sẽ do OTel làm.

Trong .NET, `Activity.Current.TraceId` làm đúng việc này một cách tự động.

## 5. IP và user agent

- IP lấy từ `r.RemoteAddr` (địa chỉ TCP), bỏ phần port bằng `net.SplitHostPort`.
- **Không** tin `X-Forwarded-For`, vì client nào cũng tự đặt được header này để giả IP trong audit.
  Khi chạy sau reverse proxy thì cần một middleware "real IP" chỉ tin proxy của mình,
  giống `ForwardedHeadersOptions.KnownProxies` trong ASP.NET Core.
- User agent bị cắt còn 512 ký tự để client không nhét được chuỗi khổng lồ vào mỗi dòng audit.
- Ngoài HTTP (worker, job) thì không có `Client`: `ClientFrom` trả về struct rỗng, và `Record` lưu NULL.

## 6. `slog.InfoContext` thay vì `slog.Info`

`slog` là logger có cấu trúc của thư viện chuẩn, tương tự `ILogger` + Serilog.
Logger của dự án (`observability.NewLogger`) có một `contextHandler` tự đọc `request_id` và `trace_id` từ ctx,
rồi thêm chúng vào mỗi dòng log. Việc này giống `BeginScope` / enricher của Serilog,
nhưng **chỉ chạy khi bạn truyền ctx**, tức là phải dùng `InfoContext(ctx, ...)` chứ không phải `Info(...)`.

```go
slog.InfoContext(ctx, "audit recorded", "audit_id", auditID, "action", e.Action, "outcome", e.OutCome)
// {"msg":"audit recorded","audit_id":"...","action":"payment.transfer","request_id":"req_…","trace_id":"4bf9…"}
```

Metadata cố ý không được log. Dòng audit trong DB mới là bản ghi chính thức; log chỉ trỏ tới nó qua `audit_id` / `request_id`.

## 7. Những lỗi trong bản nháp và cách sửa

| Bản nháp | Vấn đề | Bản sửa |
|---|---|---|
| `json.Marshal(entry.Metadata)` trước khi sanitize | PIN/password **được lưu nguyên** vào DB | `SanitizeMetadata` trước, rồi mới marshal |
| `db.WithTx` bên trong `Record` | Thừa: Record đã yêu cầu tx có sẵn, và `WithTx` lúc này chỉ "join" | Bỏ đi |
| Marshal/unmarshal cả `entry` qua `MetadataFrom` | Chạy vòng, dễ sai | Chỉ sanitize `Metadata` |
| Bỏ qua lỗi `InsertAuditLogs` | INSERT lỗi mà vẫn báo thành công | `if err != nil { return fmt.Errorf(...: %w, err) }` |
| `&entry.Reason`, `&entry.Ip` với chuỗi rỗng | Lưu `""` thay vì NULL | `nilIfEmpty`, `nilIfZero` |
| Caller tự điền `RequestId`, `TraceId`, `Ip`, `UserAgent` | Có thể quên hoặc điền sai | Bỏ khỏi `Entry`, đọc từ ctx |

`%w` trong `fmt.Errorf` bọc lỗi gốc, giống `new Exception(msg, inner)`. `errors.Is(err, target)` thì
tương tự kiểm tra `InnerException` theo chuỗi.

## 8. Chạy test

```bash
make test                                          # unit: parse traceparent, middleware, logger trace_id
make test-integration RUN='Audit|NotInTransaction' # đọc lại dòng audit, actor SYSTEM, rollback
```

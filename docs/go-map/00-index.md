# Bản đồ Go cho dev C# — wallet-core

Bộ tài liệu này giúp bạn đọc được và sửa được repo này khi mới bắt đầu với Go. Mọi khái niệm đều được neo vào **code thật** của repo (link `file:line`). Mỗi chương có bài tập chạy được trong `learn/`.

## Lộ trình 2 ngày

| Buổi | Đọc | Làm |
|---|---|---|
| Ngày 1 sáng | [01 Cú pháp](01-cu-phap.md), [02 Kiểu dữ liệu](02-kieu-du-lieu.md) | `go test ./learn/01_syntax/ ./learn/02_types/ -v`, rồi làm bài tập (xóa `t.Skip`) |
| Ngày 1 chiều | [03 Hàm, method, interface](03-ham-method-interface.md), [04 Package & module](04-package-module.md) | Bài tập 03, 04; đọc `internal/app/app.go` |
| Ngày 2 sáng | [05 Error handling](05-error-handling.md), [06 Concurrency](06-concurrency.md) | `go test -race ./learn/06_concurrency/ -v`; bài tập 05, 06 |
| Ngày 2 chiều | [07 Testing](07-testing.md), [08 Dependency](08-dependency-management.md), [09 Tour một request](09-tour-transfer.md) | Chạy end-to-end (bên dưới), đặt breakpoint theo chương 09 |

## Chạy thử toàn hệ thống (5 phút)

```bash
cp .env.example .env
make up migrate-up seed     # Postgres :5433, tạo bảng, tạo ví alice/bob có 1.000.000 ₫
make run-api                # terminal 1
make run-worker             # terminal 2

# terminal 3 — lấy wallet_id của bob từ output của `make seed`
curl -s -X POST localhost:8080/v1/transfers \
  -H 'X-Dev-Customer-Id: 0199a000-0000-7000-8000-00000000000a' \
  -H 'Idempotency-Key: demo-1' -H 'Content-Type: application/json' \
  -d '{"recipient":{"wallet_id":"wal_..."},"amount":200000,"currency":"VND"}'
```

Gửi lại đúng lệnh đó: bạn sẽ nhận **cùng** `payment_id`, vì idempotency đã chặn lần chuyển thứ hai. Đổi `amount` mà giữ nguyên key thì nhận `422 IDEMPOTENCY_KEY_REUSED`. Ở terminal 2, worker sẽ log `notify payment succeeded (mock)`.

## Cheat-sheet lệnh `go` (so với `dotnet`)

| Việc | Go | .NET tương đương |
|---|---|---|
| Build | `go build ./...` | `dotnet build` |
| Chạy | `go run ./cmd/api` | `dotnet run --project src/Api` |
| Test tất cả | `go test ./...` | `dotnet test` |
| Test một hàm | `go test ./internal/ledger/domain -run TestApply -v` | `dotnet test --filter Name~TestApply` |
| Test một subtest | `-run 'TestPostingValidate/unbalanced'` | — |
| Race detector | `go test -race ./...` | (không có sẵn) |
| Format | `gofmt -w .` (bắt buộc, không tranh cãi style) | `dotnet format` |
| Lint cơ bản | `go vet ./...` | Roslyn analyzers |
| Thêm package | `go get github.com/x/y@v1.2.3` | `dotnet add package` |
| Dọn dependency | `go mod tidy` | — |
| Xem docs | `go doc ./internal/ledger Service.Post` | F12 / XML docs |
| Tool của dự án | `go tool sqlc generate` | `dotnet tool run` |

`./...` nghĩa là "package hiện tại và mọi package con".

## Đọc repo theo thứ tự nào

1. [`internal/app/app.go`](../../internal/app/app.go): composition root, chỗ ghép mọi thứ lại, tương đương `Program.cs` + DI registration.
2. [`internal/payment/http/handler.go`](../../internal/payment/http/handler.go): controller.
3. [`internal/payment/service.go`](../../internal/payment/service.go): application service, điều phối giao dịch.
4. [`internal/ledger/service.go`](../../internal/ledger/service.go) và [`internal/ledger/domain/domain.go`](../../internal/ledger/domain/domain.go): domain thuần.
5. [`internal/platform/db/db.go`](../../internal/platform/db/db.go): transaction xuyên module.
6. [`internal/platform/outbox/`](../../internal/platform/outbox/): event và worker.

Kiến trúc tổng thể nằm ở `../E-Wallet/docs/architecture.md`, còn quy ước repo ở [`CLAUDE.md`](../../CLAUDE.md).

## Bảng khái niệm tổng hợp C# → Go

| C# | Go | Chương |
|---|---|---|
| `class` | `struct` + method | 02, 03 |
| `interface IFoo` + `: IFoo` | `interface` được thỏa mãn **ngầm định** | 03 |
| `public` / `private` | Chữ **Hoa** / chữ thường đầu tên | 04 |
| `namespace`, assembly | package (thư mục), module (`go.mod`) | 04 |
| `internal` | thư mục `internal/` | 04 |
| exception, `try/catch` | `error` là giá trị trả về, `if err != nil` | 05 |
| `Task`, `async/await` | goroutine + channel; code viết đồng bộ | 06 |
| `CancellationToken` | `context.Context` | 06 |
| `lock` | `sync.Mutex` | 06 |
| xUnit `[Theory]` | table-driven test + `t.Run` | 07 |
| NuGet, `.csproj` | `go.mod`, `go.sum` | 08 |
| DI container | Truyền dependency qua constructor bằng tay | 03 |
| `null` | `nil` (chỉ với pointer, slice, map, chan, func, interface) | 02 |
| `enum` | `type X string/int` + `const` (+ `iota`) | 02 |
| Generics `<T>` | `[T constraint]` | 02 |
| `using`/`IDisposable` | `defer x.Close()` | 01 |

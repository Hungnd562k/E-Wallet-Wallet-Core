# 04 — Package, module và ranh giới kiến trúc

Bài tập: [`learn/04_packages`](../../learn/04_packages/).

## Khái niệm

| Go | C# gần nhất | Ghi chú |
|---|---|---|
| **module** (`go.mod`) | solution/repo + NuGet package | Có một import path gốc: `github.com/Hungnd562k/E-Wallet-Wallet-Core` |
| **package** = một thư mục | namespace + assembly nhỏ | Mọi file `.go` trong cùng thư mục là cùng package, nhìn thấy cả phần unexported của nhau |
| Import path | `using` | `import ".../internal/ledger"` rồi dùng `ledger.Service`. Tên package là tiền tố bắt buộc |
| Chữ **Hoa** đầu tên | `public` | Áp dụng cho type, func, const, var, field, method |
| chữ thường | `private` (trong package) | Không có `protected` |
| Thư mục `internal/` | `internal` | Chỉ code nằm dưới **cha** của `internal/` mới import được |
| `package foo_test` | test project riêng | Black-box test, chỉ thấy phần exported |
| `func init()` | static constructor | Tránh dùng nếu có thể |

Go **cấm import vòng** giữa các package: nếu A import B thì B không được import A. Nhờ vậy kiến trúc phân lớp được compiler ép một phần.

Tên package nên ngắn, là danh từ, viết thường, không có `_`. Tránh các tên chung chung như `util` hay `common`. Khi gọi `ledger.Post`, cái tên đã đủ nghĩa, không cần `ledger.LedgerPost`.

## Kiến trúc của repo dùng package thế nào

```text
internal/<module>/service.go   ← API public duy nhất của module
internal/<module>/domain/      ← logic thuần, private
internal/<module>/store/       ← code sqlc sinh, private
internal/<module>/http/        ← transport, chỉ app/cmd import
internal/platform/<x>/         ← hạ tầng dùng chung, không phụ thuộc module nào
internal/app/                  ← composition root
cmd/<binary>/main.go           ← package main
```

Compiler chỉ biết luật `internal/`. Các luật còn lại (module A chỉ được gọi `service.go` của module B, `ledger` không được import `payment`...) được **test kiến trúc** ép: [`internal/archtest/arch_test.go:88`](../../internal/archtest/arch_test.go#L88). Test này chạy `go list -json ./...`, duyệt import graph rồi so với allowlist ở [dòng 20](../../internal/archtest/arch_test.go#L20). Muốn thấy nó hoạt động, hãy thêm vào `internal/wallet` một import tới `internal/merchant`, chạy `make test`, đọc thông báo lỗi, rồi xóa import đi.

## Thấy ở đâu trong repo

- **Tên package khác tên thư mục**: thư mục `internal/payment/http` nhưng `package paymenthttp` ([`handler.go:6`](../../internal/payment/http/handler.go#L6)), để không trùng với `net/http`. Khi import, đặt alias: [`internal/app/app.go:21`](../../internal/app/app.go#L21).
- **Re-export qua alias** để caller không phải import `domain`: [`internal/ledger/service.go:26`](../../internal/ledger/service.go#L26).
- **Package `main`** là điểm vào của mỗi binary: `cmd/api`, `cmd/worker`, `cmd/migrate`, `cmd/devseed`.
- **`//go:embed`** nhúng file SQL vào binary lúc compile: [`migrations/migrations.go:17`](../../migrations/migrations.go#L17).
- **Blank import để lấy side-effect** (driver tự đăng ký trong `init`): `_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"` trong `migrations.go`.
- **Unexported trong package, exported ra ngoài**: `errNotInTx` và `ErrInsufficientFunds` ở [`internal/ledger/service.go:46`](../../internal/ledger/service.go#L46).

## Bài tập

1. Đọc [`learn/04_packages/bank`](../../learn/04_packages/bank/). Trong `bank_test.go`, bỏ comment dòng `acc.balance = 1` và đọc lỗi compile.
2. Tạo `learn/04_packages/bank/internal/audit/audit.go` có hàm `Record(msg string)`, rồi gọi nó từ `bank.Open`. Sau đó thử import package đó từ `learn/01_syntax`: compiler báo gì?
3. Chạy `go list -deps ./internal/payment | grep E-Wallet` để xem `payment` phụ thuộc những package nội bộ nào.

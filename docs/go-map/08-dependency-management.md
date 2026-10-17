# 08 — Quản lý dependency

Bài tập: [`learn/08_deps`](../../learn/08_deps/).

## `go.mod` và `go.sum`

| File | Vai trò | C# tương đương |
|---|---|---|
| [`go.mod`](../../go.mod) | Tên module, phiên bản Go, danh sách dependency kèm version | `.csproj` `<PackageReference>` |
| `go.sum` | Hash của từng module, để phát hiện bị sửa đổi | `packages.lock.json` |

Commit **cả hai** file.

```text
module github.com/Hungnd562k/E-Wallet-Wallet-Core
go 1.26.0                      ← phiên bản Go tối thiểu (golang.org/x/sync và sqlc yêu cầu 1.26)
require ( ... )                ← dependency trực tiếp
require ( ... // indirect )    ← dependency bắc cầu
tool github.com/sqlc-dev/sqlc/cmd/sqlc   ← tool của dự án, chạy bằng `go tool sqlc`
```

## Lệnh thường dùng

```bash
go get github.com/go-chi/chi/v5@v5.5.0     # thêm hoặc nâng version
go get github.com/go-chi/chi/v5@latest
go get -u ./...                            # nâng mọi dependency (cẩn thận)
go mod tidy                                # thêm thứ còn thiếu, xóa thứ không dùng (chạy trước mỗi commit)
go mod why github.com/google/uuid          # vì sao cần package này
go list -m all                             # toàn bộ module trong build
go list -m -u all                          # module nào có bản mới
go version -m ./path/to/binary             # dependency đã nhúng trong binary
go get -tool github.com/x/y/cmd/z          # thêm tool (Go 1.24+), chạy bằng `go tool z`
```

## Khác biệt quan trọng so với NuGet

- **Semantic import versioning**: major version ≥ 2 nằm **trong import path**, ví dụ `github.com/jackc/pgx/v5`. Vì vậy v4 và v5 có thể cùng tồn tại trong một build.
- **Minimal Version Selection**: Go chọn version **thấp nhất** thỏa mọi yêu cầu, không tự lấy bản mới nhất. Build vì thế ổn định và lặp lại được.
- **Không có registry trung tâm bắt buộc**: import path chính là URL của repo. Mặc định tải qua proxy `proxy.golang.org`, và checksum được xác minh với `sum.golang.org`.
- **Thư viện chuẩn rất rộng**: HTTP server, JSON, crypto, SQL, log có cấu trúc (`log/slog`), test... Repo này chỉ cần thêm vài dependency.

## Dependency của repo và lý do

| Module | Dùng cho | ADR |
|---|---|---|
| `github.com/go-chi/chi/v5` | Router HTTP, tương thích `net/http` | ADR-04 |
| `github.com/jackc/pgx/v5` | Driver Postgres + pool | ADR-05 |
| `github.com/sqlc-dev/sqlc` (tool) | Sinh code Go type-safe từ SQL | ADR-05 |
| `github.com/golang-migrate/migrate/v4` | Chạy migration (dạng thư viện, file SQL được embed) | — |
| `github.com/google/uuid` | UUIDv7 | ADR-06 |
| `golang.org/x/sync` | `errgroup` | — |
| `github.com/testcontainers/testcontainers-go` | Postgres tạm cho integration test | — |

`golang.org/x/...` là các thư viện "bán chuẩn" do chính Go team duy trì.

## Bài tập

1. Chạy `go mod why github.com/jackc/pgx/v5` và `go mod graph | grep testcontainers | head`.
2. Build `cmd/api` rồi chạy `go version -m` trên binary. Có thấy testcontainers trong danh sách không, và vì sao? (Gợi ý: nó chỉ được import trong file `_test.go`.)
3. Thêm thử `github.com/shopspring/decimal`, dùng nó trong `learn/08_deps`, chạy `go mod tidy` và xem `go.mod` thay đổi thế nào. Sau đó xóa đi và `go mod tidy` lại.

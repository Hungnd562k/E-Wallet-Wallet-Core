# 07 — Testing

Bài tập: [`learn/07_testing`](../../learn/07_testing/).

## Cơ bản

- File test có hậu tố `_test.go`, nằm **cạnh** code được test. Hàm test có dạng `func TestXxx(t *testing.T)`.
- Không cần framework: `testing` có sẵn trong thư viện chuẩn. Assertion chỉ là `if got != want { t.Errorf(...) }`.
- `t.Errorf` ghi nhận lỗi rồi **chạy tiếp**. `t.Fatalf` ghi nhận lỗi rồi **dừng** test ngay.
- `package foo` là white-box test, thấy được cả phần unexported. `package foo_test` là black-box test.

| xUnit / C# | Go |
|---|---|
| `[Fact]` | `func TestX(t *testing.T)` |
| `[Theory] + [InlineData]` | Table-driven test: slice các struct + vòng `for` + `t.Run(name, ...)` |
| `Assert.Equal` | `if got != want { t.Errorf("got %v, want %v", got, want) }` |
| `IClassFixture` / setup | `TestMain(m *testing.M)` cho cả package; helper `newEnv(t)` cho từng test |
| `IDisposable` trong test | `t.Cleanup(func(){...})` |
| `[Trait("Category","Integration")]` | `go test -short` + `if testing.Short() { t.Skip() }` |
| Moq | Fake viết tay thỏa interface |
| BenchmarkDotNet | `func BenchmarkX(b *testing.B)` + `go test -bench .` |
| (không có sẵn) | Fuzzing: `func FuzzX(f *testing.F)` + `go test -fuzz` |
| (không có sẵn) | `Example` function: vừa là test vừa là tài liệu |

## Chạy test

```bash
make test                                   # unit + archtest + learn (go test -short ./...)
make test-integration                       # Postgres thật qua testcontainers, -race
go test ./internal/ledger/domain -run TestPostingValidate -v
go test ./internal/ledger/domain -run 'TestPostingValidate/unbalanced' -v   # một subtest
make test-one PKG=./test/... RUN=TestConcurrentSameKey
go test -count=1 ./...                      # bỏ cache kết quả test
go test -cover ./internal/...               # % coverage
make cover                                  # mở coverage.html
```

## Kim tự tháp test của repo

| Lớp | Ở đâu | Cần gì |
|---|---|---|
| Unit (domain thuần) | `internal/*/domain/*_test.go`, `platform/*/..._test.go` | Không cần gì |
| Kiến trúc | [`internal/archtest`](../../internal/archtest/arch_test.go) | `go` CLI |
| Integration | [`test/integration`](../../test/integration/) | Docker (testcontainers) |
| Bài học | `learn/` | Không cần gì |

## Thấy ở đâu trong repo

- **Table-driven + `t.Run`**: [`internal/ledger/domain/domain_test.go:13`](../../internal/ledger/domain/domain_test.go#L13).
- **Biến thể "mutate"**: mỗi case sửa một bản copy của input hợp lệ: [`internal/payment/domain/domain_test.go:33`](../../internal/payment/domain/domain_test.go#L33). Chú ý `tr := valid` copy struct (chương 02).
- **`TestMain` dựng Postgres một lần** cho cả package: [`test/integration/main_test.go:26`](../../test/integration/main_test.go#L26).
- **Mỗi test tự cô lập** bằng dữ liệu mới (UUID mới), không truncate bảng: `newWallet` trong [`helpers_test.go`](../../test/integration/helpers_test.go).
- **Test HTTP end-to-end** bằng `httptest.NewServer(app.NewAPIHandler(...))`. Đây là lý do repo có package `internal/app`: test dựng **đúng** handler mà production dùng.
- **Fake clock** để kiểm tra backoff mà không cần `sleep`: [`test/integration/outbox_test.go:60`](../../test/integration/outbox_test.go#L60).
- **Test bất biến ở tầng DB** bằng cách bỏ qua app, ghi SQL trực tiếp: [`test/integration/ledger_test.go:28`](../../test/integration/ledger_test.go#L28).
- **Tách phần ngẫu nhiên ra để test cho chắc**: `Backoff` gọi `backoffWithJitter(attempt, r)`, và test truyền `r` cố định ([`internal/platform/outbox/backoff.go:16`](../../internal/platform/outbox/backoff.go#L16)).

## Bài tập

1. Đọc mọi test trong [`learn/07_testing/testing_test.go`](../../learn/07_testing/testing_test.go). Chạy benchmark (`go test ./learn/07_testing -bench . -benchmem`) và fuzz trong 10 giây (`-fuzz FuzzNormalizePhone -fuzztime 10s`).
2. Làm bài tập ở cuối file: số máy bàn Hà Nội có bị nhận nhầm là số di động không?
3. Viết unit test cho `wallet/domain`: mọi trạng thái đều không chuyển được sang chính nó.
4. Viết integration test `TestTransferFromInactiveWallet`: tạo ví nhưng **không** `Activate`, chuyển tiền, và mong đợi `403 WALLET_NOT_ACTIVE`.

# 01 — Cú pháp cơ bản

Bài tập: [`learn/01_syntax`](../../learn/01_syntax/). Chạy bằng `go test ./learn/01_syntax -v`.

## So sánh nhanh

| C# | Go |
|---|---|
| `var x = 5;` / `int x = 5;` | `x := 5` (trong hàm) / `var x int = 5` / `var x int` (zero value) |
| `const int Max = 10;` | `const Max = 10` (untyped constant) |
| `if (a > b) { }` | `if a > b { }`: không ngoặc tròn, **bắt buộc** ngoặc nhọn |
| `if (int.TryParse(s, out var n))` | `if n, err := strconv.Atoi(s); err == nil { }`: khai báo ngay trong `if` |
| `for`, `while`, `foreach` | chỉ có `for`: `for i := 0; i < n; i++`, `for cond {}`, `for {}`, `for i, v := range xs` |
| `switch` cần `break` | `switch` **không** fall-through mặc định, `case` có thể là điều kiện |
| `using var f = ...;` | `defer f.Close()`: chạy khi hàm return, theo thứ tự LIFO |
| `public`/`private` | Tên bắt đầu bằng chữ Hoa = exported |
| Dấu `;` | Không cần, compiler tự chèn. Hệ quả: `{` phải nằm **cùng dòng** |
| Biến không dùng → warning | Biến/import không dùng → **lỗi compile** |
| `default(T)` | **Zero value**: `0`, `""`, `false`, `nil`. Mọi biến luôn được khởi tạo |

## Những điểm hay vấp

- **`:=` và `=`.** `:=` khai báo biến mới, `=` gán cho biến đã có. Viết `a, err := f()` lần hai trong cùng scope vẫn được, miễn có **ít nhất một** biến mới ở vế trái.
- **Shadowing.** Viết `err := ...` bên trong một block `if` sẽ tạo ra `err` **mới** che `err` bên ngoài. `go vet` bắt được một số trường hợp.
- **Một hàm có thể trả nhiều giá trị**: `func f() (int, error)`. Đây là nền tảng của error handling (chương 05).
- **Named result** `func f() (total int64)` cho phép `return` trống. Chỉ nên dùng cho hàm ngắn, hoặc khi cần sửa kết quả trong `defer` (xem `WithTx` bên dưới).
- **`_`** dùng để bỏ qua một giá trị: `for _, v := range xs`.

## Thấy ở đâu trong repo

- **Hàm `main` chuẩn của Go**: [`cmd/api/main.go:26`](../../cmd/api/main.go#L26). `main()` chỉ gọi `run()`; mọi lỗi được trả về một chỗ rồi `os.Exit(1)`. Lý do là `defer` **không** chạy khi gọi `os.Exit`, nên logic nằm trong `run()` để `defer pool.Close()` ([dòng 47](../../cmd/api/main.go#L47)) luôn chạy.
- **Khai báo trong `if`**: trong `config.Load` ([`internal/platform/config/config.go:38`](../../internal/platform/config/config.go#L38)), dạng `if err := ...; err != nil`.
- **`switch` không có biểu thức**, dùng như chuỗi `if/else`: [`internal/payment/domain/domain.go:76`](../../internal/payment/domain/domain.go#L76).
- **`defer` + `recover` + named result**: [`internal/platform/db/db.go:44`](../../internal/platform/db/db.go#L44). `WithTx` khai báo `(err error)` để hàm `defer` đọc được lỗi cuối cùng và rollback.
- **`range` trên slice, lấy địa chỉ phần tử**: [`internal/ledger/service.go:177`](../../internal/ledger/service.go#L177). `acc := &rows[i]` trỏ thẳng vào phần tử trong slice; còn `for _, acc := range rows` sẽ cho bạn một **bản sao**.
- **Literal số có `_`**: `1_000_000` trong [`cmd/devseed/main.go`](../../cmd/devseed/main.go).

## Bài tập

1. Viết `ParseVND` trong [`learn/01_syntax/syntax.go`](../../learn/01_syntax/syntax.go), xóa dòng `t.Skip` trong test, rồi chạy `go test ./learn/01_syntax -run TestParseVND -v`.
2. Thêm một `case` vào `TestFormatVND` với số âm lớn (`-1234567`). Test có pass không?
3. Chạy `go doc ./learn/01_syntax FormatVND` để thấy doc comment được dùng làm tài liệu.

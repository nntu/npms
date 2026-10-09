# NPMS

**Network Printer Management System** — nền tảng quản lý máy in trong mạng LAN,
tập trung vào SNMP, counter và triển khai độc lập trên Linux hoặc Windows.

NPMS v1 dùng Go, React và SQLite. Không bắt buộc Docker, PostgreSQL hoặc message broker.
Bản tiếng Anh được giữ tại [README.en.md](README.en.md).

## Tổng quan

NPMS cung cấp:

- Device registry và endpoint SNMP bằng UUID ổn định.
- SNMP v2c/v3, ưu tiên v3 `authPriv`.
- CLI chẩn đoán GET/WALK và console check.
- Probe discovery một IP cụ thể để đọc identity máy in.
- Profile máy in bằng YAML bên ngoài binary.
- Polling có timeout, retry và giới hạn concurrency.
- Lưu raw reading bất biến và kiểm tra chất lượng counter.
- REST API dưới `/api/v1` và React dashboard.
- Biểu đồ lịch sử raw counter theo thời gian, có hiển thị unit và quality.
- Quy đổi delta sử dụng theo ngày và tổng hợp theo tháng với timezone IANA và quality rõ ràng.
- Đăng ký printer, credential và primary endpoint trong một transaction.
- Binary hợp nhất chạy migration, API, worker và frontend.

Usage ngày được materialize trong SQLite sau mỗi chu kỳ counter poll. Đặt
`report.timezone` trong `config.yaml` bằng một IANA timezone, ví dụ
`Asia/Ho_Chi_Minh`; API và dashboard dùng các bản tổng hợp này để báo cáo
theo ngày/tháng mà không phải quét lại toàn bộ raw readings.

Worker tự động dọn lịch sử vận hành theo `retention.*`: chỉ xóa poll run cũ
không còn raw reading tham chiếu, counter event cũ và job đã hoàn tất. Raw
readings, usage tổng hợp và job đang chạy được giữ lại.

Luồng xử lý:

```text
SNMP adapter -> profile -> counter validation -> SQLite -> REST API/frontend
```

Nguyên tắc bảo vệ dữ liệu:

- Không dùng IP hoặc hostname làm khóa chính.
- Không ghi credential vào log hoặc API response.
- Timeout không được biến thành counter bằng 0.
- Counter giảm được xử lý như reset/wrap nghi ngờ.
- Profile không được chạy code hoặc gọi lệnh ngoài.

## Phạm vi

### Có trong v1

- Quản lý thiết bị và credential SNMP.
- Polling trạng thái và counter.
- SQLite migration/repository.
- Profile generic Printer-MIB và profile theo model.
- REST API, frontend và triển khai standalone.

### Không có trong v1

- USB agent, Windows/Linux background agent.
- CUPS hoặc IPP transport.
- Dynamic plugin, message broker hoặc microservice.
- Quét mạng không giới hạn hoặc quét Internet.

## Cấu trúc dự án

```text
backend/cmd/api/          HTTP API server (có nhúng Web frontend)
backend/cmd/worker/       Polling worker
backend/cmd/init/         CLI khởi tạo cấu hình (npms-init)
backend/cmd/db-migrate/   SQLite migration
backend/cmd/snmp-debug/   CLI chẩn đoán SNMP
backend/internal/         Domain, SNMP, profile, polling, repository
backend/profiles/         YAML profile bên ngoài binary
frontend/                 React + TypeScript + Vite
docs/                     Tài liệu API và vận hành
scripts/                  Build standalone Linux/Windows
```

## Yêu cầu

- Go phiên bản được dự án hỗ trợ.
- Node.js 20.19+ LTS để build frontend.
- Mạng LAN cho phép UDP/161 tới máy in.

SQLite dùng driver pure-Go nên binary Linux/Windows không cần PostgreSQL,
Docker hoặc CGO runtime.

## Cấu hình

Tạo cấu hình local tự động (tự sinh encryption_key 32-byte an toàn):

```bash
# Sử dụng Makefile:
make init-config

# Hoặc dùng CLI npms:
./npms init

# Hoặc dùng binary npms-init:
./bin/npms-init --generate-api-token
```

Hoặc sao chép thủ công từ file mẫu:

```bash
cp config.example.yaml config.yaml
```

Windows PowerShell:

```powershell
Copy-Item ./config.example.yaml ./config.yaml
```


Các trường quan trọng:

```yaml
database:
  path: ./data/npms.db
server:
  listen: 127.0.0.1:8080
  allowed_origin: http://localhost:8080
  api_token: ""
  # Tự động mở trình duyệt mặc định khi khởi động xong
  open_browser: true
security:
  encryption_key: thay-bang-khoa-32-byte-base64-hoac-64-ky-tu-hex
polling:
  concurrency: 5
  status_interval: 5m
  counter_interval: 15m
report:
  timezone: Asia/Ho_Chi_Minh
retention:
  polling_runs_days: 30
  counter_events_days: 90
  jobs_days: 30
  cleanup_interval: 24h
logging:
  error_file: ./data/npms-errors.log
profiles:
  path: ./profiles
```


`database.path` và `profiles.path` được tính tương đối từ vị trí `config.yaml`.
`logging.error_file` cũng được tính tương đối và chỉ ghi log mức error dạng
JSON để phục vụ debug. `logging.daily: true` tạo file hậu tố ngày như
`npms-errors-2026-10-08.log`; khi vượt `logging.max_size_mb`, file được tách
thành các phần `.part-001`, `.part-002`. Backend runtime không yêu cầu ENV.
Không commit config chứa secret thật.
Khi đóng gói chỉ binary, chép thư mục profile đi kèm và cập nhật `profiles.path`.

## Chạy nhanh

Linux/macOS:

```bash
cp config.example.yaml config.yaml
make migrate-up
make test
make lint
```

Windows PowerShell:

```powershell
Set-Location backend
go run ./cmd/db-migrate --config ../config.yaml
go test ./...
go fmt ./...
go vet ./...
```

## Kiểm tra SNMP

Sử dụng CLI chẩn đoán `snmp-debug` để kiểm tra kết nối SNMP v2c/v3 và đọc counter máy in mà không ghi dữ liệu vào SQLite:

Windows PowerShell (dùng Binary đã build):
```powershell
.\dist\bin\npms-snmp-debug.exe check --host 192.168.1.20 --version 2c --community public
```

Linux / Go source:
```bash
./dist/bin/npms-snmp-debug check --host 192.168.1.20 --version 2c --community public
# Hoặc chạy trực tiếp từ source:
cd backend && go run ./cmd/snmp-debug check --host 192.168.1.20 --version 2c --community public
```

SNMPv3 sử dụng thêm các tham số `--username`, `--auth-protocol`, `--auth-passphrase`, `--priv-protocol` và `--priv-passphrase`. Hướng dẫn chi tiết đầy đủ tất cả 5 lệnh (`check`, `probe`, `get`, `walk`, `export`) xem tại [docs/snmp-debug.md](docs/snmp-debug.md).

Frontend cũng hỗ trợ chức năng `Probe SNMP` cho một IP duy nhất qua API `POST /api/v1/discovery/probe`. Không tự động lưu device và không lưu credential probe.

## Profile máy in

Profile là YAML bên ngoài binary, được API, worker và binary hợp nhất validate
khi khởi động. Profile hiện có:

- Generic Printer-MIB.
- HP LaserJet Pro M402dn.
- HP LaserJet Enterprise M501dn.
- Brother HL-L5100DN/T.
- Brother HL-L6210DW/T.

Profile model hiện là `experimental`. Chỉ chuyển sang `verified` sau khi có
SNMP WALK ẩn danh, xác nhận index/unit và đối chiếu với configuration page
hoặc panel máy in.

Xem [docs/snmp-profiles.md](docs/snmp-profiles.md).

## Triển khai Linux

```bash
cp config.example.yaml config.yaml
mkdir -p bin
cd backend
go mod download
go build -trimpath -ldflags="-s -w" -o ../bin/npms-api ./cmd/api
go build -trimpath -ldflags="-s -w" -o ../bin/npms-worker ./cmd/worker
go build -trimpath -ldflags="-s -w" -o ../bin/npms-db-migrate ./cmd/db-migrate
cd ../frontend
npm ci
npm run build
cd ..
./bin/npms-db-migrate --config ./config.yaml
./bin/npms-api --config ./config.yaml
```

Chạy worker ở process/service riêng:

```bash
./bin/npms-worker --config ./config.yaml
```

### Một binary

```bash
./scripts/build-standalone.sh
cp config.example.yaml config.yaml
./npms --config ./config.yaml
```

## Triển khai Windows

PowerShell từ thư mục gốc:

```powershell
Copy-Item ./config.example.yaml ./config.yaml
New-Item -ItemType Directory -Force ./bin | Out-Null
Set-Location backend
go mod download
go build -trimpath -ldflags="-s -w" -o ../bin/npms-api.exe ./cmd/api
go build -trimpath -ldflags="-s -w" -o ../bin/npms-worker.exe ./cmd/worker
go build -trimpath -ldflags="-s -w" -o ../bin/npms-db-migrate.exe ./cmd/db-migrate
Set-Location ..
npm --prefix frontend ci
npm --prefix frontend run build
./bin/npms-db-migrate.exe --config ./config.yaml
./bin/npms-api.exe --config ./config.yaml
```

### Một binary

```powershell
./scripts/build-standalone.ps1
Copy-Item ./config.example.yaml ./config.yaml
./npms.exe --config ./config.yaml
```

Frontend được nhúng trong binary hợp nhất; SQLite và profiles vẫn nằm bên ngoài.

## Kiểm thử

Backend:

```bash
cd backend
go test ./...
go vet ./...
```

Frontend:

```bash
cd frontend
npm ci
npm run typecheck
npm run lint
npm run build
```

Test mặc định không cần máy in thật. Kiểm thử phần cứng phải dùng mạng được
ủy quyền và fixture ẩn danh.

## Tài liệu

- [README.en.md](README.en.md) — bản tiếng Anh.
- [docs/platforms.md](docs/platforms.md) — Linux/Windows.
- [docs/snmp-debug.md](docs/snmp-debug.md) — CLI SNMP.
- [docs/snmp-profiles.md](docs/snmp-profiles.md) — profile và schema.
- [docs/database.md](docs/database.md) — SQLite, migration và backup.
- [docs/openapi.yaml](docs/openapi.yaml) — hợp đồng API.
- [docs/frontend.md](docs/frontend.md) — frontend.

## Giới hạn hiện tại

- Profile model vẫn ở trạng thái `experimental/unverified`.
- Tự động tạo counter definition từ profile cần thêm fixture và quy tắc xác minh.
- Không coi marker-life là pages/sheets nếu chưa có bằng chứng model.
- Không quét mạng ngoài subnet hoặc danh sách được cấp quyền.

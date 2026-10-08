# NPMS — Hệ thống quản lý máy in qua mạng

NPMS là modular monolith viết bằng Go và React, dùng SNMP để quản lý máy in
trong mạng LAN. Bản v1 ưu tiên triển khai độc lập trên Linux hoặc Windows,
dùng SQLite, không yêu cầu Docker, PostgreSQL hay message broker.

Phiên bản tiếng Anh được lưu tại [README.en.md](README.en.md).

## Phạm vi v1

- Đăng ký máy in và endpoint SNMP bằng UUID ổn định.
- SNMP v2c và v3; ưu tiên v3 `authPriv`.
- Chẩn đoán SNMP GET/WALK bằng CLI.
- Profile YAML generic và profile theo model.
- Polling giới hạn đồng thời, retry và timeout rõ ràng.
- Lưu raw reading bất biến, kiểm tra counter tăng/giảm/reset/spike.
- REST API xác thực dưới prefix `/api/v1`.
- Dashboard React/TypeScript.
- Chạy độc lập bằng SQLite và một binary hợp nhất.

Không nằm trong v1: USB agent, Windows agent, Linux agent, CUPS, IPP,
dynamic plugin, message broker hoặc quét mạng Internet.

## Kiến trúc

Hệ thống là modular monolith với các executable chính:

- `npms`: chạy migration, API, worker và phục vụ frontend trong một binary.
- `api`: chỉ chạy HTTP API.
- `worker`: polling status/counter.
- `db-migrate`: chạy migration SQLite.
- `snmp-debug`: kiểm tra SNMP trực tiếp từ console.

Luồng xử lý:

```text
SNMP adapter -> profile -> counter validation -> repository -> API/frontend
```

Domain không phụ thuộc HTTP, database driver hoặc transport SNMP. Credential
được mã hóa khi lưu; không ghi community string hay passphrase vào log/API.

## Profile máy in

Profile là file YAML bên ngoài binary, được nạp từ `profiles.path` trong
`config.yaml`. Có thể thêm hoặc cập nhật profile mà không build lại executable.

Profile hiện có:

- Generic Printer-MIB.
- HP LaserJet Pro M402dn.
- HP LaserJet Enterprise M501dn.
- Brother HL-L5100DN/T.
- Brother HL-L6210DW/T.

Các profile model hiện có trạng thái `experimental`. Chúng dùng marker-life
generic của Printer-MIB và chưa khẳng định marker là số trang hay số tờ.
Chỉ chuyển sang `verified` sau khi có SNMP WALK ẩn danh và đối chiếu với bảng
cấu hình/màn hình máy in.

Chi tiết tại [docs/snmp-profiles.md](docs/snmp-profiles.md).

## Yêu cầu

- Go phiên bản được dự án hỗ trợ.
- Node.js 20.19+ LTS cho frontend.
- Mạng LAN cho phép UDP/161 tới máy in khi kiểm thử SNMP.

SQLite dùng driver pure-Go nên binary Linux/Windows không cần CGO runtime.

## Cấu hình

Sao chép file mẫu:

```bash
cp config.example.yaml config.yaml
```

PowerShell:

```powershell
Copy-Item .\config.example.yaml .\config.yaml
```

Các nhóm cấu hình chính:

```yaml
database:
  path: ./data/npms.db
server:
  listen: 127.0.0.1:8080
  allowed_origin: http://localhost:8080
  api_token: ""
security:
  encryption_key: thay-bang-khoa-32-byte-base64-hoac-64-ky-tu-hex
polling:
  concurrency: 5
  status_interval: 5m
  counter_interval: 15m
profiles:
  path: ./backend/profiles
```

`database.path` và `profiles.path` tương đối theo vị trí file cấu hình.
Không commit `config.yaml`, credential SNMP, API token hoặc encryption key thật.
Trong triển khai chỉ có binary, hãy chép thư mục `profiles/` đi kèm binary và
đổi `profiles.path` cho phù hợp.

## Phát triển cục bộ

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
go run ./cmd/db-migrate --config ..\config.yaml
go test ./...
go fmt ./...
go vet ./...
```

Không cần khởi động Docker. Tất cả dữ liệu nằm trong file SQLite theo
`database.path`.

## Kiểm tra SNMP từ console

Lệnh `check` hiển thị IP/host, thông tin cơ bản và giá trị marker hiện tại;
không ghi vào SQLite:

```bash
cd backend
go run ./cmd/snmp-debug check \
  --host 192.168.1.20 \
  --version 2c \
  --community public
```

SNMPv3:

```bash
go run ./cmd/snmp-debug check \
  --host 192.168.1.20 \
  --version 3 \
  --username snmp-user \
  --auth-protocol SHA \
  --auth-passphrase '***' \
  --priv-protocol AES \
  --priv-passphrase '***'
```

Không đưa credential thật vào shell history hoặc file commit.

## Build và triển khai Linux

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

Chạy worker ở terminal/service khác:

```bash
./bin/npms-worker --config ./config.yaml
```

Có thể đăng ký API và worker bằng systemd hoặc service manager của hệ thống.
Chỉ mở mạng LAN khi cần và giới hạn UDP/161 tới các IP máy in được phép.

## Build và triển khai Windows

PowerShell từ thư mục gốc:

```powershell
Copy-Item .\config.example.yaml .\config.yaml
New-Item -ItemType Directory -Force .\bin | Out-Null
Set-Location backend
go mod download
go build -trimpath -ldflags="-s -w" -o ..\bin\npms-api.exe .\cmd\api
go build -trimpath -ldflags="-s -w" -o ..\bin\npms-worker.exe .\cmd\worker
go build -trimpath -ldflags="-s -w" -o ..\bin\npms-db-migrate.exe .\cmd\db-migrate
Set-Location ..
npm --prefix frontend ci
npm --prefix frontend run build
.\bin\npms-db-migrate.exe --config .\config.yaml
.\bin\npms-api.exe --config .\config.yaml
```

Worker chạy ở cửa sổ PowerShell khác hoặc đăng ký bằng Windows Service.
Không chia sẻ file SQLite qua network share.

## Một binary cho máy đơn

Script build sẽ build frontend, chép frontend vào embedded assets và tạo một
binary chạy migration, API, worker cùng HTTP server:

Linux/macOS:

```bash
./scripts/build-standalone.sh
cp config.example.yaml config.yaml
./npms --config ./config.yaml
```

Windows:

```powershell
.\scripts\build-standalone.ps1
Copy-Item .\config.example.yaml .\config.yaml
.\npms.exe --config .\config.yaml
```

Mở `http://127.0.0.1:8080`. Frontend được nhúng trong binary nhưng SQLite và
thư mục profile vẫn nằm bên ngoài để backup/cập nhật độc lập.

## Kiểm thử và chất lượng

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
ủy quyền và fixture ẩn danh. Không đánh dấu profile `verified` nếu chưa đối
chiếu counter với máy in thực tế.

## Tài liệu

- [README.en.md](README.en.md) — bản tiếng Anh.
- [docs/platforms.md](docs/platforms.md) — hướng dẫn Linux/Windows.
- [docs/snmp-debug.md](docs/snmp-debug.md) — CLI SNMP.
- [docs/snmp-profiles.md](docs/snmp-profiles.md) — schema và profile.
- [docs/database.md](docs/database.md) — SQLite, migration và backup.
- [docs/openapi.yaml](docs/openapi.yaml) — hợp đồng API.

## Giới hạn hiện tại

- Profile model mới ở trạng thái `experimental/unverified`.
- Tự động tạo counter definition từ profile vẫn cần fixture và quy tắc chọn
  marker được xác minh.
- Không hỗ trợ USB, CUPS hoặc IPP trong v1.
- Không chạy scanner mạng ngoài danh sách/subnet được cho phép.


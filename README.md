# Sudosyu Agent

Agent monitoring yang berjalan di setiap server. Mengumpulkan metrik CPU, RAM, Disk, Network, dan Docker, lalu mengirimkan ke Sudosyu Core.

## Requirements

- Docker & Docker Compose
- Akses ke Docker socket (`/var/run/docker.sock`) — opsional, untuk monitoring container
- Koneksi ke Sudosyu Core backend

## Deploy

### 1. Clone

```bash
git clone https://github.com/ivanadhii/sudosyu-agent.git
cd sudosyu-agent
```

### 2. Buat config

Salin contoh config dan sesuaikan:

```bash
cp config.yaml.example config.yaml
```

Edit `config.yaml`:

```yaml
backend_url: "http://<ip-core-server>:8080"
api_key: "api-key-dari-dashboard"
server_name: "nama-server-ini"
interval_seconds: 10
docker_socket: "/var/run/docker.sock"
```

| Field | Keterangan |
|---|---|
| `backend_url` | URL backend Sudosyu Core |
| `api_key` | Per-server key atau Super key dari dashboard |
| `server_name` | Nama server yang tampil di dashboard. Wajib diisi jika pakai super key. |
| `interval_seconds` | Interval kirim data (detik). Rekomendasi: 5–10 |
| `docker_socket` | Path Docker socket. Hapus baris ini jika tidak ada Docker. |

### 3. Jalankan

```bash
docker compose up -d
```

### 4. Update

```bash
git pull
docker compose up --build -d
```

---

## Pakai Super Key (banyak server)

Jika punya banyak server, gunakan satu **Super Key** dari dashboard (Settings → Super Keys). Server akan otomatis terdaftar di dashboard berdasarkan `server_name`.

Tidak perlu buat per-server key satu-satu.

---

## Metrik yang Dikumpulkan

| Kategori | Detail |
|---|---|
| CPU | Total %, per-core %, load average 1/5/15 menit |
| RAM | Used %, used GB, total GB, available GB, swap |
| Disk | Used % dan ukuran per mount point |
| Disk I/O | Read/write bytes per detik, IOPS |
| Network | Bytes in/out per detik, packets in/out per detik |
| Docker | Status, CPU %, memory, network, block I/O per container |
| Docker DF | Ukuran image, container, volume, build cache |

---

## Tanpa Docker

Jika server tidak pakai Docker, cukup hapus baris `docker_socket` dari `config.yaml` dan hapus volume mount di `docker-compose.yml`:

```yaml
# Hapus baris ini dari docker-compose.yml:
- /var/run/docker.sock:/var/run/docker.sock:ro
```

---

## Struktur

```
agent/
├── cmd/            # Entrypoint
├── collector/      # Pengumpul metrik (CPU, RAM, Disk, Docker, dll)
├── sender/         # HTTP client ke backend
├── config/         # Parsing config.yaml
├── config.yaml     # Konfigurasi aktif (tidak di-commit)
├── config.yaml.example
└── docker-compose.yml
```

# Virtual Exam API

Backend API สำหรับแพลตฟอร์มจำลองสอบเสมือนจริง (Thai-style multiple-choice exams with bubble answer sheets).

## Tech Stack

- Go 1.23
- Echo
- GORM + PostgreSQL
- Redis
- JWT Auth

## Quick Start (Docker Compose)

```bash
cd api
cp .env.example .env
docker compose up --build
```

API: `http://localhost:8080`  
Health: `GET /health`  
Base URL: `http://localhost:8080/api/v1`

## Local Development (without Docker)

1. Start PostgreSQL and Redis locally
2. Copy env file:

```bash
cp .env.example .env
```

3. Run migrations + seed + server:

```bash
go run ./cmd/server
```

Or seed only:

```bash
go run ./cmd/seed
```

## Frontend Integration

Set in Next.js:

```env
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080/api/v1
```

CORS allows `http://localhost:3000` by default.

Demo exam set code for frontend: `demo`

## Demo Credentials

| Email | Password | Role |
|-------|----------|------|
| admin@example.com | password123 | admin |
| demo@example.com | password123 | user |

## Admin

Admin UI lives in the Next.js app at `/admin` (requires `role = admin`).

### Create admin user

Fresh database with seed (`AUTO_SEED=true` or `go run ./cmd/seed`) includes `admin@example.com`.

To promote an existing user manually:

```sql
UPDATE users SET role = 'admin' WHERE email = 'admin@example.com';
```

### Login as admin

1. Open `http://localhost:3000/login`
2. Email: `admin@example.com` / Password: `password123`
3. Navigate to `http://localhost:3000/admin`

Non-admin users see a forbidden page at `/admin/*`.

### Admin routes (frontend)

| Path |
|------|
| /admin |
| /admin/exam-tracks |
| /admin/exam-sets |
| /admin/subjects |
| /admin/questions |
| /admin/exam-sets/:id/questions |

### Admin API (prefix `/api/v1/admin`, JWT + admin role required)

| Method | Path |
|--------|------|
| GET | /dashboard |
| GET/POST | /exam-tracks |
| GET/PUT/DELETE | /exam-tracks/:id |
| GET/POST | /exam-sets |
| GET/PUT/DELETE | /exam-sets/:id |
| GET/POST/PUT reorder/DELETE | /exam-sets/:id/questions |
| GET/POST | /subjects |
| GET/PUT/DELETE | /subjects/:id |
| GET/POST | /questions |
| GET/PUT/DELETE | /questions/:id |

### Example curl

Login admin:

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"password123"}'
```

Create exam track:

```bash
curl -X POST http://localhost:8080/api/v1/admin/exam-tracks \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"name":"สอบ ก.พ.","code":"gpor","description":"เตรียมสอบ ก.พ.","is_active":true}'
```

Create subject:

```bash
curl -X POST http://localhost:8080/api/v1/admin/subjects \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"name":"กฎหมายราชการ","code":"law","description":"หมวดกฎหมายราชการ"}'
```

## API Endpoints

| Method | Path | Auth |
|--------|------|------|
| POST | /api/v1/auth/register | No |
| POST | /api/v1/auth/login | No |
| GET | /api/v1/auth/me | Yes |
| GET | /api/v1/home | Optional |
| GET | /api/v1/exam-tracks | No |
| GET | /api/v1/exam-tracks/:trackCode | No |
| GET | /api/v1/exam-tracks/:trackCode/exam-sets | No |
| GET | /api/v1/exam-sets | No |
| GET | /api/v1/exam-sets/:examSetCode | No |
| GET | /api/v1/exam-sets/:examSetCode/questions-preview | No |
| POST | /api/v1/exam-sets/:examSetCode/attempts | Yes |
| GET | /api/v1/attempts/:attemptId | Yes |
| PUT | /api/v1/attempts/:attemptId/answers/:questionNo | Yes |
| DELETE | /api/v1/attempts/:attemptId/answers/:questionNo | Yes |
| POST | /api/v1/attempts/:attemptId/submit | Yes |
| GET | /api/v1/attempts/:attemptId/result | Yes |
| GET | /api/v1/attempts/:attemptId/review | Yes |
| GET | /api/v1/me/results/summary | Yes |
| GET | /api/v1/me/results/exam-tracks | Yes |
| GET | /api/v1/me/results/exam-tracks/:trackCode | Yes |
| GET | /api/v1/me/results | Yes |
| GET | /api/v1/me/results/exam-sets/:examSetCode | Yes |

## Example cURL Commands

### 1. Register

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "display_name": "Demo User",
    "email": "user@example.com",
    "password": "password123"
  }' | jq
```

### 2. Login

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "demo@example.com",
    "password": "password123"
  }' | jq -r '.data.access_token')

echo $TOKEN
```

### 3. Get Home

```bash
curl -s http://localhost:8080/api/v1/home \
  -H "Authorization: Bearer $TOKEN" | jq
```

### 4. Get Exam Tracks

```bash
curl -s http://localhost:8080/api/v1/exam-tracks | jq
```

### 5. Get Exam Sets

```bash
curl -s "http://localhost:8080/api/v1/exam-sets?access_type=free&page=1&limit=10" | jq
```

### 6. Start Attempt

```bash
ATTEMPT=$(curl -s -X POST http://localhost:8080/api/v1/exam-sets/demo/attempts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" | jq -r '.data.attempt_id')

echo $ATTEMPT
```

### 7. Save Answer

```bash
curl -s -X PUT "http://localhost:8080/api/v1/attempts/$ATTEMPT/answers/1" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"selected_choice_key": "B"}' | jq
```

### 8. Submit Attempt

```bash
curl -s -X POST "http://localhost:8080/api/v1/attempts/$ATTEMPT/submit" \
  -H "Authorization: Bearer $TOKEN" | jq
```

### 9. Get Result

```bash
curl -s "http://localhost:8080/api/v1/attempts/$ATTEMPT/result" \
  -H "Authorization: Bearer $TOKEN" | jq
```

### 10. Get Review

```bash
curl -s "http://localhost:8080/api/v1/attempts/$ATTEMPT/review" \
  -H "Authorization: Bearer $TOKEN" | jq
```

### 11. My Results Summary

```bash
curl -s http://localhost:8080/api/v1/me/results/summary \
  -H "Authorization: Bearer $TOKEN" | jq
```

### 12. My Results By Exam Track

```bash
curl -s http://localhost:8080/api/v1/me/results/exam-tracks \
  -H "Authorization: Bearer $TOKEN" | jq
```

### 13. My Results Track Detail

```bash
curl -s http://localhost:8080/api/v1/me/results/exam-tracks/gpor \
  -H "Authorization: Bearer $TOKEN" | jq
```

### 14. My Attempt History

```bash
curl -s "http://localhost:8080/api/v1/me/results?limit=20&page=1" \
  -H "Authorization: Bearer $TOKEN" | jq
```

### 15. My Exam Set Result Detail

```bash
curl -s http://localhost:8080/api/v1/me/results/exam-sets/gpor-set-1 \
  -H "Authorization: Bearer $TOKEN" | jq
```

## Project Structure

```
api/
├── cmd/server/          # HTTP server entrypoint
├── cmd/seed/            # Seed command
├── internal/            # Clean architecture modules
├── migrations/          # SQL migrations
├── seed/                # Seed logic
├── docker-compose.yml
└── Dockerfile
```

## Seed Data

- Exam tracks: สอบ ก.พ., สอบตำรวจ, สอบท้องถิ่น, สอบครูผู้ช่วย
- Exam sets with cover image and pricing:
  - `gpor-set-1` — ฟรี, cover image, featured
  - `gpor-set-2` — Premium ฿199 (ลดเหลือ ฿149), featured
  - `police-set-1` — Premium ฿249
  - `local-set-1` — ฟรี
  - `demo` — ฟรี, Official, featured
- 20 questions seeded per set (display `total_questions` may show 80–100)
- Demo user: demo@example.com / password123
- Demo submitted attempts (for `/my-results`):
  - ก.พ. ชุดที่ 1: 55%, 72%, 80%
  - ก.พ. ชุดที่ 2: 61%, 74%
  - ตร. ชุดที่ 1: 58%
  - ท้องถิ่น ชุดที่ 1: 70%

### Seed demo exam sets (cover + price)

Fresh database (AutoMigrate + AutoSeed on server start):

```bash
go run ./cmd/server
```

Or seed only (skips if data already exists — drop DB to re-seed):

```bash
go run ./cmd/seed
```

SQL migration for pricing fields: `migrations/000002_exam_set_pricing.up.sql`

### Test start-exam flow

1. Login and obtain token (see cURL below)
2. `GET /exam-sets/gpor-set-1` — verify cover image and price fields
3. `POST /exam-sets/gpor-set-1/attempts` — creates attempt (premium stub: authenticated users allowed)
4. Frontend: `/exams` → card → `/exams/gpor-set-1` → instruction modal → `/exams/gpor-set-1/take?attempt_id=...`

### Exam set API examples

```bash
# List exam sets (with cover + price)
curl -s http://localhost:8080/api/v1/exam-sets | jq

# Get single exam set
curl -s http://localhost:8080/api/v1/exam-sets/gpor-set-1 | jq

# Start attempt (requires auth)
curl -s -X POST http://localhost:8080/api/v1/exam-sets/gpor-set-1/attempts \
  -H "Authorization: Bearer $TOKEN" | jq
```

## Response Format

Success:

```json
{ "data": {} }
```

Error:

```json
{
  "error": {
    "code": "ATTEMPT_NOT_FOUND",
    "message": "ไม่พบข้อมูลการสอบ"
  }
}
```

## Notes

- PostgreSQL is the source of truth; Redis caches attempt answers during exam
- Premium payment is **not implemented** — UI shows prices; authenticated users may start premium sets (stub)
- Leaderboard is not implemented in this phase

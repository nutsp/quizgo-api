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

### Admin workflow

1. Create subjects (`/admin/subjects`)
2. Create or import questions (`/admin/questions`, `/admin/questions/import`)
3. Create exam set (`/admin/exam-sets/new`)
4. Bulk assign questions to exam set (`/admin/exam-sets/:id/questions`)
5. Preview public exam detail (`/exams/:examSetCode`)
6. Start test attempt (public API or `/exams/:examSetCode/take`)

### Admin exam set questions API

| Method | Path | Description |
|--------|------|-------------|
| GET | /exam-sets/:id/available-questions | List question bank with filters |
| GET | /exam-sets/:id/questions | List assigned questions + exam set summary |
| POST | /exam-sets/:id/questions/bulk | Bulk add questions to exam set |
| POST | /exam-sets/:id/questions | Add single question (legacy) |
| PUT | /exam-sets/:id/questions/reorder | Reorder assigned questions |
| DELETE | /exam-sets/:id/questions/:questionId | Remove one question |
| DELETE | /exam-sets/:id/questions | Clear all questions (requires `{"confirm":true}`) |

**Bulk add (curl)**

```bash
curl -X POST http://localhost:8080/api/v1/admin/exam-sets/<EXAM_SET_ID>/questions/bulk \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"question_ids":["<Q1>","<Q2>"],"score":1,"append_to_end":true}'
```

**List assigned (curl)**

```bash
curl http://localhost:8080/api/v1/admin/exam-sets/<EXAM_SET_ID>/questions \
  -H "Authorization: Bearer <TOKEN>"
```

**List available questions (curl)**

```bash
curl "http://localhost:8080/api/v1/admin/exam-sets/<EXAM_SET_ID>/available-questions?exclude_assigned=true&status=published&page=1&limit=20" \
  -H "Authorization: Bearer <TOKEN>"
```

If the exam set has submitted attempts, add/remove/reorder returns:

```json
{
  "error": {
    "code": "EXAM_SET_LOCKED_BY_ATTEMPTS",
    "message": "ชุดข้อสอบนี้มีผลสอบแล้ว ไม่สามารถแก้ไขคำถามในชุดได้"
  }
}
```

### Admin routes (frontend)

| Path |
|------|
| /admin |
| /admin/exam-tracks |
| /admin/exam-sets |
| /admin/subjects |
| /admin/questions |
| /admin/questions/import |
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
| GET | /exam-sets/:id/available-questions |
| POST | /exam-sets/:id/questions/bulk |
| GET/POST | /subjects |
| GET/PUT/DELETE | /subjects/:id |
| GET/POST | /questions |
| GET/PUT/DELETE | /questions/:id |
| GET | /questions/import/template |
| POST | /questions/import/preview |
| POST | /questions/import/confirm |

### Admin Question Import

Admin can bulk-import questions from CSV or Excel into the question bank at `/admin/questions/import`.

**Template columns**

| Column | Required | Notes |
|--------|----------|-------|
| subject_code | Yes | Must match an existing subject code (e.g. `law`, `math`) |
| question_text | Yes | At least 5 characters |
| choice_a … choice_d | Yes | Choice text for ก ข ค ง |
| correct_choice | Yes | `A`–`D` or `ก`–`ง` |
| explanation | No | Recommended |
| difficulty | No | `easy`, `medium`, `hard` (default: `medium`) |
| status | No | `draft`, `published`, `archived` (default: `draft`) |

**Example CSV**

```csv
subject_code,question_text,choice_a,choice_b,choice_c,choice_d,correct_choice,explanation,difficulty,status
law,"ข้อใดเป็นหนังสือราชการภายนอก","บันทึกข้อความ","หนังสือภายนอก","หนังสือสั่งการ","หนังสือประชาสัมพันธ์","B","หนังสือภายนอกใช้สำหรับติดต่อระหว่างส่วนราชการ",medium,published
math,"5 + 7 เท่ากับข้อใด","10","11","12","13","C","5 + 7 = 12",easy,published
```

**How to import**

1. Log in as admin and open `/admin/questions/import`
2. Download the CSV template
3. Fill in your questions and upload the file
4. Click **Preview ข้อมูล** — review valid/invalid rows and warnings
5. Confirm import (optionally import only valid rows if some rows have errors)
6. Imported questions appear in `/admin/questions`

**Common validation errors**

| Message | Cause |
|---------|-------|
| ไม่พบคอลัมน์ subject_code | Missing required column in header |
| ไม่พบหมวดวิชานี้ในระบบ | `subject_code` does not exist |
| กรุณาระบุคำถาม | Empty question text |
| กรุณาระบุตัวเลือก ก/ข/ค/ง | Empty choice |
| เฉลยต้องเป็น A, B, C, D หรือ ก, ข, ค, ง | Invalid correct_choice |
| ระดับความยากไม่ถูกต้อง | Invalid difficulty value |
| สถานะไม่ถูกต้อง | Invalid status value |

Warnings (non-blocking): missing explanation, duplicate question in file, question already exists in database.

**Download template (curl)**

```bash
curl -OJ http://localhost:8080/api/v1/admin/questions/import/template \
  -H "Authorization: Bearer $TOKEN"
```

**Preview upload (curl)**

```bash
curl -X POST http://localhost:8080/api/v1/admin/questions/import/preview \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@questions.csv"
```

**Confirm import (curl)**

```bash
curl -X POST http://localhost:8080/api/v1/admin/questions/import/confirm \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"import_id":"<UUID>","import_only_valid_rows":true}'
```

SQL migration: `migrations/000004_question_import.up.sql`

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

## Exam Set Publish Workflow

Admin workflow for making exam sets visible to users on **สนามสอบเสมือนจริง**:

1. **Create exam set** — `POST /api/v1/admin/exam-sets` (status starts as `draft`)
2. **Assign questions** — add published questions via `/admin/exam-sets/:id/questions/bulk`
3. **Check readiness** — `GET /api/v1/admin/exam-sets/:id/readiness`
4. **Preview** — `GET /api/v1/admin/exam-sets/:id/preview` or admin UI at `/admin/exam-sets/:id/preview`
5. **Publish** — `POST /api/v1/admin/exam-sets/:id/publish`
6. Public users see the set on `/exams` and can start attempts

Exam set status values:

| Status | Meaning |
|--------|---------|
| `draft` | Admin is editing; hidden from users |
| `published` | Visible and startable (`status = published` AND `is_active = true`) |
| `archived` | Hidden from users; history preserved |

Readiness check (admin only):

```bash
curl http://localhost:8080/api/v1/admin/exam-sets/<ID>/readiness \
  -H "Authorization: Bearer $TOKEN"
```

Publish:

```bash
curl -X POST http://localhost:8080/api/v1/admin/exam-sets/<ID>/publish \
  -H "Authorization: Bearer $TOKEN"
```

Unpublish (returns to draft, blocks new attempts):

```bash
curl -X POST http://localhost:8080/api/v1/admin/exam-sets/<ID>/unpublish \
  -H "Authorization: Bearer $TOKEN"
```

Archive:

```bash
curl -X POST http://localhost:8080/api/v1/admin/exam-sets/<ID>/archive \
  -H "Authorization: Bearer $TOKEN"
```

Starting an attempt on an unpublished set returns `EXAM_SET_NOT_PUBLISHED`.

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

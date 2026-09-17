# Help portal — Koyeb smoke test

Мінімальний, **неproduction**-стенд для перевірки деплою на Koyeb: Go + Chi API та статичний Svelte UI в одному Docker-контейнері.

## Що перевіряє

- Koyeb збирає multi-stage Dockerfile;
- контейнер слухає змінну `PORT`;
- `GET /healthz` повертає `200`;
- Svelte UI звертається до Go API;
- `POST /api/sections` працює (дані зберігаються лише в RAM).

## Локально

Потрібні Go 1.27+ і Node 24+.

```powershell
cd C:\Go\help-portal-koyeb-smoke\web
npm install
npm run build

cd ..
go mod tidy
go run .
```

Відкрити `http://localhost:8000`; перевірка доступності: `http://localhost:8000/healthz`.

Для розробки UI в іншому вікні:

```powershell
cd C:\Go\help-portal-koyeb-smoke\web
npm run dev
```

## Koyeb

1. Створити новий GitHub-репозиторій і запушити цей каталог.
2. У Koyeb: **Create Web Service → GitHub → Dockerfile**.
3. Обрати `free` instance, Frankfurt, expose port `8000` (Koyeb також передає `PORT`).
4. Після деплою відкрити `/healthz`, потім головну сторінку.

Не додавайте Supabase-змінні: цей стенд навмисно працює без БД, Storage і справжньої авторизації. Якщо він стабільно задеплоїться, наступним кроком буде заміна RAM-store на Supabase та перенесення екранів з `help-portal`.

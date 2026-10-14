# RareQC Registre Admin — Development Guide

## Project overview
Web application for administrators of a Quebec rare disease patient registry. Manages patient records, diagnoses, and registry metadata.

## Architecture
- **Backend**: Go 1.26 + Gin + GORM + PostgreSQL + Swagger (swaggo)
- **Frontend**: React 19 + TypeScript + Vite + Tailwind CSS v4 + shadcn/ui + i18next
- **Infrastructure**: Docker Compose (PostgreSQL 16, Go API, Nginx frontend)

## Backend conventions
- Module name: `registre-admin`
- Entry point: `backend/cmd/api/main.go`
- Layers: `database` → `repository` (DAO interfaces) → `server` (Gin handlers) → `types`
- Handler files: `handlers_<domain>.go` (e.g., `handlers_participants.go`)
- GORM models go in `internal/types/`
- Migrations: `golang-migrate` with versioned SQL files in `backend/migrations/`
- Swagger annotations on handler functions, generated via `swag init`
- Table names are **singular** in PostgreSQL (e.g., `participant`, `contact`, `consent`)
- Business constants (sex_at_birth, vital_status, relationship, etc.) come from PostgreSQL reference tables, not hardcoded in frontend. Exception: provinces can stay in the frontend.

## Backend API endpoints

### Participants
| Method | Route | Description |
|--------|-------|-------------|
| GET | `/participants` | List (paginated, sortable, searchable, consent status filter) |
| GET | `/participants/:id` | Get single participant with contacts and GUID |
| POST | `/participants` | Create participant |
| PUT | `/participants/:id` | Update participant |
| DELETE | `/participants/:id` | Delete participant (cascade; audited as `participant_deleted`) |
| POST | `/participants/resolve-ids` | Resolve a list of ids/RAMQ/external ids to participant ids (bulk filter) |

### Contacts
| Method | Route | Description |
|--------|-------|-------------|
| POST | `/participants/:id/contacts` | Add contact |
| PUT | `/contacts/:contactId` | Update contact |
| DELETE | `/contacts/:contactId` | Delete contact |

### Consents
| Method | Route | Description |
|--------|-------|-------------|
| GET | `/participants/:id/consents` | List consents for a participant |
| POST | `/participants/:id/consents` | Create consent (blocked if same clause type exists) |
| PUT | `/consents/:consentId` | Update consent |
| GET | `/consent-clauses` | List clauses (optional `template_document_id` filter) |
| GET | `/consent-templates` | List templates (includes `has_consents` flag) |
| POST | `/consent-templates` | Create template with PDF + clauses |
| PUT | `/consent-templates/:id` | Update template (blocked if signed by participants) |
| DELETE | `/consent-templates/:id` | Delete template (blocked if signed by participants) |

### External IDs
| Method | Route | Description |
|--------|-------|-------------|
| GET | `/participants/:id/external-ids` | List external system IDs for a participant |

### Communications
| Method | Route | Description |
|--------|-------|-------------|
| GET | `/participants/:id/communications` | List communications for a participant |
| POST | `/participants/:id/communications` | Record a communication (phone/email attempt) |
| PUT | `/communications/:communicationId` | Update a communication |
| DELETE | `/communications/:communicationId` | Delete a communication |

### Cart (per user, keyed by `auth.UserSub`)
| Method | Route | Description |
|--------|-------|-------------|
| GET | `/cart/items` | List cart items with participant data |
| GET | `/cart/count` | Cart size |
| POST | `/cart/items` | Add participant ids |
| DELETE | `/cart/items` | Remove participant ids |
| DELETE | `/cart` | Clear cart |
| POST | `/cart/export-data` | Data for the Excel export (audited as `data_exported`) |
| POST | `/cart/communications` | Record a communication for every cart participant |

### Documents
| Method | Route | Description |
|--------|-------|-------------|
| POST | `/documents` | Upload document |
| GET | `/documents/:id/file` | Download document file |

### Administration (reference data)
| Method | Route | Description |
|--------|-------|-------------|
| GET | `/code-tables` | All editable code tables |
| POST | `/code-tables/:table/entries` | Add a code entry |
| PUT | `/code-tables/:table/entries/:code` | Update a code entry |
| DELETE | `/code-tables/:table/entries/:code` | Delete a code entry (409 if referenced) |
| GET/POST | `/external-systems` | List / create external systems |
| PUT/DELETE | `/external-systems/:id` | Update / delete an external system (409 if referenced) |

### Other
| Method | Route | Description |
|--------|-------|-------------|
| GET | `/health` | Health check |
| GET | `/enums` | All reference data |
| GET | `/search` | Search suggestions (participants + contacts) |
| GET | `/reports/summary` | Aggregates for the reports page |
| GET | `/activity-logs` | Activity log (paginated) |
| GET | `/participants/:id/activity-logs` | Activity log for a participant |
| GET | `/admin-users` | Keycloak accounts holding `registre_admin` (read-only, via admin API) |

## Data model key rules
- **One consent per clause type per participant** — enforced both at API level and via PostgreSQL trigger (`trg_unique_consent_clause_type`)
- **RAMQ is unique** — unique index on `participant.ramq`
- **External system IDs** — unique constraint on `(external_system_id, participant_id)`
- **Consent templates cannot be edited/deleted if signed** — API returns 409, frontend disables buttons with tooltip explanation
- **Activity log is an audit trail** — `activity_log.participant_id` is `ON DELETE SET NULL` (migration 000002), so entries outlive the participant. Audited events: participant create/edit/delete, contact create/edit/delete, consent add/edit, `data_exported` (cart export and `GET /participants?export=true`, which the Excel export sends). `participant_deleted` is written in the same transaction, before the delete; a failed audit write aborts the operation.

## Frontend conventions
- Node 22 (via nvm)
- Path alias: `@/` → `src/`
- Components: shadcn/ui in `src/components/base/ui/`, feature components in `src/components/feature/`
- **Always use shadcn/ui components** before creating custom ones. If a shadcn component exists for the need (date picker, select, dialog, etc.), use it. Never use native HTML inputs (`type="date"`, `type="file"`, etc.) directly — wrap them in shadcn components for cross-browser consistency.
- Reusable base components in `src/components/base/` (e.g., `file-upload.tsx`, `multi-select-filter.tsx`)
- i18n: French (default) and English, files in `src/locales/{fr,en}/common.json`
- API client: Axios instance in `src/lib/api.ts`, base URL `/api` (proxied by Vite dev server)
- Routing: React Router v7 with `createBrowserRouter` in `src/main.tsx`
- Badge variant mappings centralized in `src/lib/badge-variants.ts` (SEX_BADGE, VITAL_STATUS_BADGE, CONSENT_STATUS_BADGE, CONSENT_STATUS_ICON, CONSENT_STATUS_COLOR, ACTION_BADGE)
- Hooks follow `use<Entity>.ts` pattern in `src/hooks/`
- Search in participants page also searches contacts (name, email, phone) via backend subquery

### Code conventions (aligned with radiant-portal — apply to ALL new code)
- **Forms**: react-hook-form + `zodResolver` + shared `Form`/`FormField` primitives (`src/components/base/ui/form.tsx`). Zod schemas live in `src/lib/validations/` as `(t) => z.object(...)` factories using `validation.*` translation keys. Never build forms with manual per-field `useState`/`canSubmit` gates. Required-field asterisks come automatically from the schema via `isFieldRequired` (pass `schema` to `FormField`; for array-item fields pass the item schema).
- **Bilingual API fields** (`name_en`/`name_fr`, `clause_en`/`clause_fr`...): use `localizedField(obj, base, lang)` from `src/lib/enum-label.ts`. Never write `i18n.language === "en" ? x.name_en : x.name_fr` ternaries (only the navbar/landing language toggles may test `i18n.language`).
- **Dates**: display format patterns live in translation keys (`common.date.year_month_day`, `common.date.year_month_day_hour`) — keep them identical in fr/en unless asked otherwise. Use `formatDate(date, pattern?)` and `todayISO()` from `src/lib/format.ts`. Never inline `format(new Date(), "yyyy-MM-dd")` or `new Date().toISOString().slice(0, 10)`.
- **Debounce/fetch**: reuse `useDebouncedValue` + SWR with a conditional key (see `useSearch.ts`, `address-autocomplete.tsx`). Never hand-roll setTimeout debounces or raw `fetch` in effects.
- **Table columns**: pure factories `getXColumns(opts)` (no hooks inside), memoized at the consumption site with complete dependency arrays. Column cells must not close over data arrays — read rows via `table.getRowModel()`.
- **Naming**: JSX event handlers are `handle*` (`handleDelete`, `handleOpenChange`); plain verbs are reserved for pure helpers (`parseIds`, `resetForm`, `refresh`).
- **Readability**: no nested ternaries — extract a named helper function; no `while ((m = regex.exec(...)))` loops — use `splitOnHighlight` (`src/lib/highlight.ts`) or `matchAll`/`split`.
- **`src/lib/`**: dedicated single-purpose helper files (radiant style). Validation helpers go in `src/lib/validations/` (there is no `lib/validation.ts`).

## Frontend pages
- `/` — Landing page (login)
- `/home` — Dashboard with search box and navigation cards
- `/participants` — Participant list with table (consent status columns, column visibility, fullscreen, consent status filter)
- `/participants/:id` — Participant detail (identity, coordinates, contacts, consents, activity, external IDs in header badges)
- `/activity` — Activity log
- `/cart` — Bulk selection: participants added from the list, Excel export (`cart-excel-report.ts`), bulk communication
- `/communications` — Communications across participants
- `/reports` — Dashboard charts (recharts) from `/reports/summary`
- `/admin` — Administration with accordion sections (Users: read-only list of Keycloak accounts with the admin role, Consent forms: template management, code tables, external systems, participant deletion)

## Styling & theming (STRICT — replicate unic-portal / radiant-portal architecture exactly)
- **No hardcoded colors in components.** All colors must come from semantic CSS tokens via Tailwind classes (`bg-navbar`, `text-hero-foreground`, `bg-cta`, etc.). Never use brand palette classes directly (e.g., `bg-rareqc-600`, `text-slate-500`) in component code.
- **CSS architecture** (3 files):
  1. `src/index.css` — imports `tailwind.base.css` then `themes/rareqc/theme.css`
  2. `src/styles/tailwind.base.css` — Tailwind v4 import, `@custom-variant dark`, global `--radiant-*` color aliases (light/dark), `@theme inline` block exposing all semantic tokens to Tailwind, `@layer base` resets
  3. `src/styles/themes/rareqc/theme.css` — RareQC brand palette (`--color-rareqc-*` in oklch), semantic token mappings (`:root` + `.dark`)
- **Semantic token categories**: background, foreground, primary, secondary, accent, muted, neutral, destructive, card, popover, border, input, ring, sidebar-*, table-*, navbar-*, hero-*, cta-*, highlight-*, chart-*
- **When adding a new color need**: add a semantic token in `theme.css` (both `:root` and `.dark`), expose it in `tailwind.base.css` `@theme inline`, then use the Tailwind class in the component

## Database indexes
- **GIN trigram indexes** (pg_trgm) on participant and contact name/email/phone fields for `LIKE '%...%'` search performance
- **B-tree index** on `external_id.external_id` for exact lookups
- Consent status columns in participant list use a single LEFT JOIN with conditional aggregation (not correlated subqueries)

## Commands
- **Shared platform (REQUIRED first — postgres + keycloak)**: `docker compose up -d` in `~/src/rareqc-infra` (creates the external `rareqc-net` network; see that repo's README)
- **Frontend dev**: `cd frontend && npm install && npm run dev`
- **Backend dev**: `cd backend && go run ./cmd/api/`
- **Docker full stack**: `docker compose up --build` (joins `rareqc-net`)
- **Seed data**: `docker compose --profile dev run --rm seed`
- **Lint**: `cd frontend && npm run lint`
- **Format**: `cd frontend && npm run format`
- **Swagger**: `cd backend && PATH="/usr/local/go/bin:$PATH" ~/go/bin/swag init -g cmd/api/main.go --parseDependency --parseInternal`
- **PostgreSQL client**: `docker compose exec postgres psql -U rareqc -d rareqc_registre`

## Ports
| Service    | Port |
|------------|------|
| Frontend   | 5173 (dev) / 3001 (docker) |
| API        | 8080 (dev) / 8082 (docker) |
| PostgreSQL | 5440 (rareqc-infra) |
| Keycloak   | 8081 (rareqc-infra) |

## Docker notes
- Images : builder `golang:1.26-alpine` / `node:22-alpine`, runtime `alpine:3.22` (API et seed, utilisateur non-root `app`) et `nginx:1.28-alpine`. `.dockerignore` exclut `.env*`, binaires et tests. Les binaires `backend/api`/`backend/seed` produits par `go build` sont ignorés par git.
- `frontend/nginx.conf` : en-têtes de sécurité (CSP incluant le géocodeur `servicescarto.mern.gouv.qc.ca`, nosniff, DENY, no-referrer) via le snippet `nginx-security-headers.conf` inclus dans chaque location ; `client_max_body_size 11m` ; `index.html` en `no-store`, `/assets/` immuable.
- Publication : tag `v*` → workflow `publish-images` pousse `ghcr.io/<owner>/rareqc-registre-{api,seed,web}` ; `ci` (vet, tests, govulncheck, lint, typecheck, build, npm audit) sur push/PR.
- Frontend Dockerfile copies `.npmrc` (with `legacy-peer-deps=true`) before `npm ci` to handle `react-joyride` peer dependency conflict with React 19
- Seed Dockerfile copies consent PDFs from `backend/scripts/seed/` to `/data/`
- When rebuilding after code changes: `docker compose --profile dev build --no-cache seed` then run seed

## External systems
- `CQDG` — Centre québécois de données génomiques
- `CQGC` — Centre québécois de génomique clinique
- External IDs displayed as colored badges in participant detail header, sorted alphabetically by system name

## Authentication (BFF Keycloak — mirrors radiant-portal's design)
- **Pattern**: the Go backend is the confidential OIDC client (BFF). Tokens live in three signed httpOnly cookies (`session.user`, `session.token`, `session.r.token` — SameSite=Lax) and never reach browser JavaScript. Package `backend/internal/auth/`.
- **Keycloak**: realm `rareqc` hosted by the rareqc-infra platform (port 8081 dev). Roles are enforced per route group: `authService.Middleware(auth.RoleAdmin)` on admin routes (403 otherwise); future `/me/...` routes will use `auth.RoleParticipant` (constants in `internal/auth/config.go`). Test users: `vincent-test`, `marie-test` (with role), `sansrole-test` (without) — password `test1234`.
- **Endpoints**: `/api/auth/login` (code flow + PKCE S256, state cookie), `/api/auth/callback`, `/api/auth/logout` (back-channel), `/api/auth/me`. Everything else under `/api` requires auth except `/health`.
- **Dual mode**: browser = session cookies (transparent refresh on expiry); scripts/services = `Authorization: Bearer` verified against the Keycloak JWKS (client `rareqc-scripts`, flux `client_credentials` — see `rareqc-infra/scripts/api-token.sh`).
- **Identity in handlers**: `auth.UserSub(c)` (cart ownership), `auth.UserName(c)` (activity-log author via `getAuthor`).
- **Env vars** (radiant naming, dev defaults built in): `KEYCLOAK_HOST`, `KEYCLOAK_INTERNAL_HOST` (containers), `KEYCLOAK_REALM`, `KEYCLOAK_CLIENT`, `KEYCLOAK_CLIENT_SECRET`, `SESSION_SECRET`, `PORTAL_HOST`, `COOKIE_SECURE`. See `backend/.env.example`.
- **Two portals, one API (A6)**: the OIDC client is selected per request origin — origins listed in `PARTICIPANT_PORTAL_HOSTS` (default `http://localhost:5174,http://localhost:3002`) use `KEYCLOAK_PARTICIPANT_CLIENT`/`KEYCLOAK_PARTICIPANT_CLIENT_SECRET` (`portail-participant-bff`); every other origin uses the admin client. Test user: `participant-test` (role `participant`).
- **Admin users listing**: `GET /admin-users` proxies the Keycloak admin API using the `registre-admin-bff` **service account** (client_credentials), which holds only the realm-management `view-users` + `view-realm` roles (read-only — reading a role and its holders requires both).
- **Frontend**: `AuthProvider` (`src/contexts/auth-context.tsx`) loads `/api/auth/me` at startup; `RequireAuth` guards app routes; axios 401 interceptor returns to the landing page; no OIDC library needed.
- **Release guards** (`GIN_MODE=release`): besides the dev secrets, the server refuses to start unless `COOKIE_SECURE=true`, `SESSION_SECRET` is ≥ 32 chars and `KEYCLOAK_HOST`/`PORTAL_HOST`/`PARTICIPANT_PORTAL_HOSTS` are `https://`.
- **Gotchas**: nginx needs enlarged proxy buffers for the session cookies (see `frontend/nginx.conf`); Keycloak runs with `KC_HOSTNAME` + `KC_HOSTNAME_BACKCHANNEL_DYNAMIC` for the dev split-horizon.

## HTTP hardening (`internal/server/middlewares.go`)
- Engine-level: `requestID` (`X-Request-ID`, echoed in every log line), `requestLogger`, `recovery` (generic 500, stack in slog), `securityHeaders` (nosniff, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, `Cache-Control: no-store`, HSTS in release). `SetTrustedProxies` comes from `TRUSTED_PROXIES` (default: none).
- `/api` group: `bodyLimit` (1 MB JSON, 11 MB multipart — cuts the body before any handler reads it; `MaxMultipartMemory` = 10 MB) and `csrfGuard` — POST/PUT/DELETE authenticated by cookie must carry an `Origin` (or `Referer`) matching `PORTAL_HOST` or `PARTICIPANT_PORTAL_HOSTS`; `Authorization: Bearer` calls are exempt. Swagger UI from `localhost:8080` therefore needs a Bearer token for mutations.
- Swagger UI (`/swagger`) is registered only outside release, with its own relaxed CSP (`swaggerCSP`) since the API-wide `default-src 'none'` would blank the page.
- `http.Server` timeouts: header 10 s, read 60 s, write 120 s, idle 120 s, 64 KB headers.
- Errors: `handleInternalError(c, err, msg)` logs `err` with the request id and returns the generic `msg`; never send `err.Error()` to the client. Mutations go through `handleWriteError(c, err, msg)` (GORM `TranslateError` is on): unique violation → 409 (business wording per constraint in `uniqueConstraintMessages`, e.g. duplicate RAMQ), FK violation → 400 on insert/update, 409 on delete, check violation → 400, not found → 404, anything else → 500. Constraint names never reach the client; the frontend maps a 409 with `isApiConflict` (`src/lib/api-error.ts`).
- GORM logger: `ParameterizedQueries` (no bound values in logs), warnings only, 500 ms slow threshold.
- Seed script refuses to run with `GIN_MODE=release` unless `SEED_ALLOW_DESTRUCTIVE=yes`.

## TODO — Production readiness

### Backend
- [x] **B1** ~~Auth JWT (Keycloak) — remplacer `fakeUserID` et `X-Author`~~ (BFF, voir section Authentication)
- [x] **B2** ~~CORS — env var au lieu de `*`~~ (`CORS_ALLOWED_ORIGINS`, `*` refusé en release)
- [x] **B3** ~~Connection pool PostgreSQL (`MaxOpenConns`, `MaxIdleConns`)~~
- [x] **B4** ~~Graceful shutdown~~ (`signal.NotifyContext` + `srv.Shutdown`, budget 10 s)
- [x] **B5** ~~Config par env vars — refuser de démarrer si manquant~~ (`internal/config`, `GIN_MODE=release` = mode strict ; `PORT`, `POSTGRES_SSLMODE`, secrets)
- [x] **B7** ~~Remplacer AutoMigrate par `golang-migrate` (migrations SQL versionnées)~~
- [x] **B13** ~~Health check qui ping la DB~~
- [x] **B14** ~~Tests unitaires~~ (socle : `internal/auth` complet — middleware/rôles, sélection client par origine, tokens, config — + `internal/config` + patron DAO mocké dans `handlers_external_system_test.go` ; étendre aux autres handlers au fil de l'eau)
- [x] **B15** ~~Validation d'entrée (binding tags required, formats, upload 10 MB)~~
- [x] **B16** ~~Corriger les N+1 queries (templates, code tables, external systems)~~
- [x] **B17** ~~Logging structuré~~ (`slog` — JSON en release, texte en dev ; requêtes HTTP via middleware `requestLogger`, health en debug)
- [ ] **B19** Normaliser les RAMQ du seed (`scripts/seed/main.go` les écrit avec espaces alors que l'API stocke `BEAA19121472`) : l'index unique ne détecte pas un doublon entre une fiche seedée et une saisie. Vérifier de la même façon toute donnée importée en prod.
- [ ] **B18** Chiffrement au repos : disque chiffré sur la VM (LUKS/hébergeur) + sauvegardes chiffrées avant sortie de la VM ; évaluer `pgcrypto` pour la RAMQ si exigence de conformité (⚠️ casserait la recherche RAMQ actuelle). En transit DB : `sslmode` configurable (couvert par B5). TLS navigateur↔portail : couvert par A3.

### Frontend
- [x] **F12** ~~Migrer `cva` → `tv` (tailwind-variants) pour matcher radiant-portal~~
- [x] **F13** ~~Migrer `exceljs` → `4.4.0`~~ (API inchangée sur nos 4 fichiers ; override `uuid` conservé — exceljs 4.4 dépend encore d'un uuid vulnérable — mais `brace-expansion` retiré ; `npm audit` : 0 vulnérabilité)
- [x] **F14** ~~Page « accès refusé » pour les utilisateurs sans le rôle `registre_admin`~~ (`RequireAuth` vérifie le rôle ; `access-denied.tsx` avec déconnexion)

### Auth (suite)
- [x] **A1** ~~Client `portail-participant-bff` dans le realm~~ (créé, avec `participant-test` — voir `rareqc-infra/docs/integration-portail.md`)
- [x] **A2** ~~Thème de login Keycloak aux couleurs RareQC~~ (`rareqc-infra/keycloak/themes/rareqc/login/`, enfant de `keycloak.v2`, monté par le compose ; `loginTheme` posé dans le realm JSON et appliqué au realm vivant)
- [x] **A3** ~~Composition staging/prod dans rareqc-infra (proxy TLS, `KC_HOSTNAME` public, secrets régénérés, images GHCR)~~ (`rareqc-infra/docker-compose.prod.yml` + realm de prod rendu par `render-prod-realm.sh` ; images publiées par `.github/workflows/publish-images.yml` sur tag `v*`)

### Ordre suggéré des items ouverts (révision sécurité 2026-10-14)
1. **A9** — lancer la demande DSI (enregistrement de l'application Entra ID) : dépendance externe, à démarrer tôt.
2. **A7** — décision de processus (qui lie un compte Keycloak à un participant), puis la colonne `keycloak_sub`.
3. **A5** — routes `/me/...` une fois A7 tranché.
4. **B18** — à la mise en place de la VM (disque chiffré, sauvegardes chiffrées).
5. **B19** — avant tout import de données réelles.

### Auth — API partagée avec le portail participant (décision 2026-08-03 : un seul serveur API, une seule BD, voir `rareqc-infra/docs/integration-portail.md`)
- [x] **A4** ~~Rôle par groupe de routes — remplacer le `RequiredRole` global codé en dur~~ (`Middleware(role)` + constantes `auth.RoleAdmin`/`auth.RoleParticipant`)
- [ ] **A5** Routes participant `/me/...` (`/me/profile`, `/me/consents`, …) filtrées par `auth.UserSub(c)` — jamais d'accès aux listes admin ; contrat à publier dans le Swagger
- [x] **A6** ~~Sélection du client OIDC selon l'origine de la requête~~ (origines participant dans `PARTICIPANT_PORTAL_HOSTS` → client `portail-participant-bff` ; tout le reste → `registre-admin-bff`)
- [ ] **A7** Liaison compte Keycloak ↔ participant en BD (prérequis A5) : colonne `keycloak_sub` (unique, nullable) sur `participant` + décision de processus pour la remplir (association par un admin ? correspondance courriel à la première connexion ? enrôlement ?)
- [x] **A8** ~~Défense en profondeur côté portail participant (repo `rareqc-portail`) : son nginx ne doit proxifier que `/api/auth/*` et `/api/me/*` — jamais tout `/api`~~ (exigence documentée dans `rareqc-infra/docs/integration-portail.md` ; à vérifier à la revue du portail)
- [ ] **A9** Fédération d'identité pour les admins : brancher **Microsoft Entra ID** (comptes M365 du réseau de la santé) comme Identity Provider du realm `rareqc` — plus de mots de passe gérés, MFA et désactivation hérités de la DSI. Requiert l'enregistrement d'une app dans l'Entra de l'organisation (demande DSI). Attribution du rôle `registre_admin` reste manuelle (aucun rôle à l'auto-enrôlement). Précédent : CLIN fédère Microsoft (`idpHint: 'microsoft'`). ⚠️ Pas de Google/Facebook pour les participants (fuite de métadonnées de santé vers un tiers) — pour eux : comptes Keycloak + MFA, et évaluer l'Authentification gouvernementale du Québec au moment du portail participant

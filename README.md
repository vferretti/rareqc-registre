# RareQC — Registre Admin

Application web d'administration pour un registre québécois de maladies rares. Permet la gestion des participants (patients), de leurs contacts, des consentements, du suivi de l'historique des modifications, des rapports et de la sélection en lot (panier).

Le backend (API + base de données) est **partagé** avec le futur portail participant (`rareqc-portail`) : un seul serveur API, une seule base, un seul Keycloak — voir `rareqc-infra/docs/integration-portail.md`.

## Stack technique

| Couche       | Technologies                                                       |
|--------------|--------------------------------------------------------------------|
| **Backend**  | Go 1.26, Gin, GORM, PostgreSQL 16, Swagger (swaggo)               |
| **Auth**     | Keycloak (OIDC, patron BFF — cookies httpOnly, aucun jeton côté navigateur) |
| **Frontend** | React 19, TypeScript, Vite, Tailwind CSS v4, shadcn/ui, i18next, recharts, ExcelJS |
| **Infra**    | Docker Compose (Go API, Nginx) + plateforme partagée `rareqc-infra` (PostgreSQL, Keycloak) |
| **Qualité**  | ESLint, Prettier, Husky + lint-staged, tests Go, govulncheck, npm audit — GitHub Actions `ci` sur chaque push/PR |

## Prérequis

- **Plateforme partagée `rareqc-infra` démarrée** (PostgreSQL + Keycloak — crée le réseau Docker `rareqc-net`) : `docker compose up -d` dans le repo `rareqc-infra`
- [Docker](https://docs.docker.com/get-docker/) et Docker Compose
- [Go 1.26+](https://go.dev/dl/) (développement backend local)
- [Node.js 22+](https://nodejs.org/) via nvm (développement frontend local)

---

## Démarrage rapide

> **Toujours en premier** : `docker compose up -d` dans `~/src/rareqc-infra` (PostgreSQL sur 5440, Keycloak sur 8081).

### Option 1 — Stack Docker complète

```bash
# Démarrer API + Frontend (rejoignent le réseau rareqc-net)
docker compose up --build

# Charger les données de test (dans un autre terminal)
docker compose --profile dev run --rm seed
```

| Service    | URL                          |
|------------|------------------------------|
| Frontend   | http://localhost:3001         |
| API        | http://localhost:8082/api     |
| Swagger    | http://localhost:8082/swagger/index.html |
| PostgreSQL | localhost:5440 (rareqc-infra) |
| Keycloak   | http://localhost:8081 (rareqc-infra) |

### Option 2 — Développement local

```bash
# 1. L'API en dev (les défauts pointent vers la plateforme rareqc-infra — aucun .env requis)
cd backend && go run ./cmd/api/

# 2. Charger les données de test
docker compose --profile dev run --rm seed

# 3. Le frontend en mode dev
cd frontend
npm install
npm run dev
```

| Service    | URL                          |
|------------|------------------------------|
| Frontend   | http://localhost:5173         |
| API (dev)  | http://localhost:8080/api     |
| Swagger    | http://localhost:8080/swagger/index.html |

Le serveur Vite proxy `/api` vers l'API en dev local (8080). Si l'API tourne en docker plutôt qu'avec `go run` : `VITE_API_PROXY_TARGET=http://localhost:8082 npm run dev`.

### Se connecter

La page d'accueil redirige vers Keycloak (thème RareQC). Utilisateurs de test (mot de passe `test1234`) :

| Utilisateur | Rôle | Résultat |
|---|---|---|
| `vincent-test`, `marie-test` | `registre_admin` | accès complet |
| `sansrole-test` | aucun | page « accès refusé » |
| `participant-test` | `participant` | page « accès refusé » (réservé au futur portail participant) |

### Après un changement de code backend

`docker compose up --build` utilise le cache Docker. Si vous modifiez le code Go, les changements ne seront **pas** pris en compte à moins de forcer un rebuild :

```bash
# Rebuild sans cache + redémarrer
docker compose build --no-cache api && docker compose up -d api

# Puis re-seed si le schéma a changé
docker compose --profile dev run --rm seed
```

> **Pourquoi ?** Docker met en cache chaque layer du Dockerfile. Si seul le code source change mais que les layers `COPY go.mod` et `go mod download` sont identiques, Docker réutilise le binaire compilé précédemment. Le flag `--no-cache` force la recompilation complète.

### Réinitialiser la base de données

La base vit dans la composition `rareqc-infra` (volume `pgdata`, partagé avec Keycloak). Pour repartir d'un jeu de données propre, le re-seed suffit dans la plupart des cas :

```bash
docker compose --profile dev run --rm seed
```

Pour une réinitialisation complète (schéma compris), voir « Base applicative : sauvegarde / restauration » dans le README de `rareqc-infra` — ⚠️ supprimer le volume `pgdata` détruit aussi la base Keycloak (realm réimporté au prochain démarrage).

---

## Données de test (seed)

Le seed génère 200 participants réalistes répartis sur 7 trimestres (Q3 2024 → Q1 2026) avec une croissance progressive :

| Trimestre | Participants |
|-----------|:---:|
| Q3 2024 | 10 |
| Q4 2024 | 15 |
| Q1 2025 | 20 |
| Q2 2025 | 25 |
| Q3 2025 | 35 |
| Q4 2025 | 45 |
| Q1 2026 | 50 |

- **170 enfants** (0-17 ans) avec contact mère (primaire) et optionnellement père
- **30 adultes** (18-65 ans) avec contact « soi-même » (primaire)
- Noms québécois et anglophones, numéros RAMQ, codes postaux et indicatifs régionaux réalistes
- **2 formulaires de consentement** (RareQc et RQDM) avec clauses et PDFs templates
- **200 documents signés** (un par participant)
- **~400 consentements** avec statuts variés (valide, expiré, retiré) et cascade appliquée
- **2 systèmes externes** (CQDG ~60%, CQGC ~40%) avec IDs aléatoires
- **300+ entrées d'activité** réparties sur les 7 trimestres
- **4 auteurs** : John Smith, Marie Tremblay, Pierre Gagnon, Sophie Lavoie

```bash
docker compose --profile dev run --rm seed
```

Le seed est dans un profil Docker `dev` séparé et n'est jamais inclus dans l'image de production.

### Accès direct à la base de données

```bash
PGPASSWORD=rareqc psql -h localhost -p 5440 -U rareqc -d rareqc_registre
```

---

## Commandes utiles

```bash
# Tests backend
cd backend && go test ./...

# Lint frontend
cd frontend && npm run lint

# Formater le code frontend
cd frontend && npm run format

# Régénérer la spec Swagger + client TypeScript (tout-en-un)
make generate

# Ou séparément :
make doc                         # Swagger seulement (backend/docs/)
make generate-client-typescript  # Client TypeScript seulement (frontend/api/)
```

### Client API généré

Le frontend utilise un client TypeScript auto-généré depuis la spec Swagger (même pipeline que radiant-portal) :

```
Annotations Go (swag) → swagger.yaml → openapi-generator (typescript-axios) → frontend/api/
```

Après tout changement à l'API backend (nouveaux endpoints, types modifiés), exécuter `make generate` pour mettre à jour le client frontend. Les types manuels dans `src/types/` sont des barrels de ré-export depuis le client généré.

## Authentification

Patron **BFF** (backend-for-frontend) : le backend Go est le client OIDC confidentiel de Keycloak. Les jetons vivent dans trois cookies httpOnly signés et n'atteignent jamais le JavaScript du navigateur. Double mode : cookies de session (navigateur) ou `Authorization: Bearer` vérifié contre le JWKS (scripts — voir `rareqc-infra/scripts/api-token.sh`).

- Le rôle realm est exigé **par groupe de routes** : les routes admin exigent `registre_admin` ; les futures routes `/me/...` du portail participant exigeront `participant`.
- Le client OIDC est choisi **selon l'origine de la requête** : les origines listées dans `PARTICIPANT_PORTAL_HOSTS` utilisent `portail-participant-bff`, tout le reste `registre-admin-bff`.
- Un utilisateur authentifié sans le bon rôle voit une page « accès refusé » (pas d'écrans vides).

## Sécurité (résumé)

- **Transport et sessions** : BFF Keycloak, cookies httpOnly `SameSite=Lax`, garde CSRF par en-tête `Origin` sur les POST/PUT/DELETE authentifiés par cookie (les appels `Bearer` en sont exempts), `COOKIE_SECURE` et `https://` exigés en release.
- **Entrées** : identifiants de chemin validés avant toute requête SQL, corps limités (1 Mo JSON, 11 Mo multipart, 413 au-delà), erreurs PostgreSQL traduites en 409/400 sans jamais exposer les noms de contraintes, 500 génériques corrélés par `X-Request-ID` dans les journaux.
- **En-têtes** : nosniff, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, `Cache-Control: no-store`, CSP stricte sur l'API et CSP dédiée à la SPA dans nginx, HSTS en release.
- **Journal d'audit** : création, modification et suppression des participants, contacts et consentements, plus chaque export de données ; les entrées survivent à la suppression du participant.
- **Chaîne de livraison** : images non-root à bases épinglées, `govulncheck` et `npm audit` en CI, Swagger UI servi hors release seulement (les mutations depuis « Try it out » exigent un jeton Bearer, voir `rareqc-infra/scripts/api-token.sh`).

Détails dans `CLAUDE.md` (sections « Authentication » et « HTTP hardening »).

## Variables d'environnement

Tout est documenté dans [`backend/.env.example`](backend/.env.example) — les défauts pointent vers la plateforme de dev `rareqc-infra`, donc **aucun `.env` n'est requis en dev**.

| Variable | Défaut | Description |
|---|---|---|
| `GIN_MODE` | *(vide)* | `release` = mode production : **refuse de démarrer** si un secret/réglage est resté à sa valeur de dev ; logs JSON ; Swagger UI désactivé |
| `PORT` | `8080` | Port d'écoute HTTP |
| `CORS_ALLOWED_ORIGINS` | `*` | Origines CORS (séparées par des virgules) — `*` interdit en release |
| `TRUSTED_PROXIES` | *(vide)* | IPs/CIDR des proxys autorisés à poser `X-Forwarded-*` (ex. `172.16.0.0/12`) — vide = aucun, l'IP cliente est alors le pair TCP |
| `POSTGRES_HOST` / `POSTGRES_PORT` | `localhost` / `5440` | PostgreSQL (plateforme rareqc-infra) |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | `rareqc` / `rareqc` / `rareqc_registre` | Accès BD |
| `POSTGRES_SSLMODE` | `disable` | `sslmode` du driver — valeur explicite exigée en release |
| `KEYCLOAK_HOST` / `KEYCLOAK_INTERNAL_HOST` | `http://localhost:8081` / *(=HOST)* | Keycloak public / interne (conteneurs) |
| `KEYCLOAK_REALM` / `KEYCLOAK_CLIENT` / `KEYCLOAK_CLIENT_SECRET` | `rareqc` / `registre-admin-bff` / *(dev)* | Client OIDC du portail admin |
| `KEYCLOAK_PARTICIPANT_CLIENT` / `KEYCLOAK_PARTICIPANT_CLIENT_SECRET` | `portail-participant-bff` / *(dev)* | Client OIDC du portail participant |
| `PARTICIPANT_PORTAL_HOSTS` | `http://localhost:5174,http://localhost:3002` | Origines du portail participant |
| `SESSION_SECRET` | *(dev)* | Signature des cookies de session — 32 caractères minimum en release |
| `PORTAL_HOST` | `http://localhost:5173` | Origine du portail admin (redirect de secours OIDC, garde CSRF) — `https://` exigé en release |
| `COOKIE_SECURE` | `false` | Flag `Secure` des cookies — **`true` exigé en release** (comme `https://` sur `KEYCLOAK_HOST`, `PORTAL_HOST` et `PARTICIPANT_PORTAL_HOSTS`) |
| `STORAGE_TYPE` | `database` | Stockage des documents (`database` ou `s3`) |

## Déploiement (production / staging)

La composition de production vit dans le repo **rareqc-infra**
(`docker-compose.prod.yml`, Caddy TLS, realm Keycloak de production). Elle
consomme les images publiées sur GHCR par le workflow `publish-images` :

```bash
git tag v1.0.0 && git push origin v1.0.0   # → ghcr.io/<owner>/rareqc-registre-{api,seed,web}:v1.0.0
```

Puis, sur la VM, mettre `REGISTRE_IMAGE_TAG=v1.0.0` dans `.env.prod` et
relancer la composition (voir le README de rareqc-infra). En release
(`GIN_MODE=release`) l'API refuse de démarrer si un secret est resté à sa
valeur de dev, si `COOKIE_SECURE` n'est pas `true` ou si une URL n'est pas
`https://`.

## Stockage des documents

Les documents supportent deux backends de stockage :

| Mode | `STORAGE_TYPE` | Stockage | Téléchargement |
|------|----------------|----------|----------------|
| **Base de données** | `database` (défaut) | Table `document_file` (bytea) | Servi par l'API |
| **Object store** | `s3` | Bucket S3-compatible | Redirection 307 |

En développement, le mode `database` est utilisé par défaut. En production, configurer `STORAGE_TYPE=s3` avec les variables S3 appropriées.

> **Note** : L'upload vers S3 n'est pas encore implémenté. Seul le redirect au téléchargement est en place.

## Licence

Projet interne — usage restreint.

# CLIENT_ADR.md — Architekturentscheidungen des Clients

Zeitfenstermanagement · `client_vue`

Format: ADR (Architecture Decision Record) — Entscheidung, ihre Begründung und Konsequenzen.
Allgemeine Code-Regeln — `../../agentic_docs/CONVENTIONS.md`. Domäne — `../agentic_docs/MVP.md`.

> Dieses Dokument **ersetzt** §17 CONVENTIONS in Bezug auf das UI-Kit: dort war PrimeVue angegeben,
> hier wurde Bootstrap gewählt (ADR-003).

---

## ADR-001: Vue 3, Composition API, `<script setup>`

**Kontext.** Ein Frontend für die Demo-Konsole wird benötigt (drei Panels + Drawer, siehe MVP §8).
Der Entwickler kommt aus der React-Welt.

**Entscheidung.** Vue 3, ausschließlich Composition API, ausschließlich `<script setup lang="ts">`.
Die Options API wird nicht verwendet.

**Warum.** Die Composition API ist den React-Hooks näher als die Options API — der Umstieg ist
günstiger (Spickzettel im Anhang). `<script setup>` eliminiert `defineComponent`, `return {...}`
und sonstigen Boilerplate: Alles, was im Script deklariert wird, ist automatisch im Template
verfügbar.

**Konsequenzen.** Viele Online-Beispiele verwenden die Options API — diese müssen gedanklich
übersetzt werden. Die Options API ist im Projekt verboten, damit es keinen Dialekt-Mix in
derselben Codebasis gibt.

---

## ADR-002: TypeScript strict

**Entscheidung.** `strict: true`, `any` ist verboten. API-Typen werden aus `api/openapi.yaml`
generiert und nicht manuell geschrieben.

**Warum.** Der Vertrag mit dem Backend ist die einzige Quelle der Wahrheit. Ein manuell
geschriebenes `interface Buchung` weicht in der zweiten Woche vom Server ab, ohne dass es
jemand bemerkt — bis es zur Laufzeit abstürzt.

**Konsequenzen.** Wird ein Handler in Go geändert, muss `openapi.yaml` aktualisiert und die
Typen neu generiert werden. Das ist Disziplin, aber sie erkennt Abweichungen zur Kompilierzeit.
In §15 CONVENTIONS gibt es bereits den Checklistenpunkt „`api/openapi.yaml` ist synchron mit
den Handlern".

---

## ADR-003: Bootstrap 5. Kein Tailwind

**Entscheidung.** Bootstrap 5. Tailwind wird in keiner Form verwendet.

**Warum.**

- Bootstrap ist der Standard im deutschen Enterprise-Umfeld, und Nagel bildet keine Ausnahme.
  Interne Systeme sehen genau so aus. Ein Projekt in ihrer Domäne sollte wie ihr Werkzeug
  aussehen, nicht wie die Landing Page eines Startups.
- Fertige Komponentensemantik (`.btn`, `.card`, `.table`, `.offcanvas`) — keine eigene
  Design-System-Erfindung für ein Pet-Projekt notwendig.
- Tailwinds Utility-Klassen in Vue-Templates erzeugen Zeilen mit 200 Zeichen; das ist ein
  eigener Geschmack, den wir nicht teilen.

**Konsequenzen.** Das Aussehen wird erkennbar Bootstrap-typisch sein. Das ist eine **Funktion,
kein Fehler** — das Ziel ist nicht, mit Design zu beeindrucken, sondern wie ein
Enterprise-Werkzeug auszusehen.

**Ehrlicher Vorbehalt.** Bootstrap hilft nicht beim Hauptelement des Bildschirms. Sein Grid
hat 12 Spalten, der `Tagesplan`-Timeline jedoch 34 Spalten (05:00–22:00 im 30-Minuten-Raster)
× N Rampen. Das ist ein **eigenes CSS Grid**, und das ist die aufwändigste CSS-Arbeit im
Projekt. Bootstrap liefert Buttons, Formulare, Tabellen und das Gerüst — aber kein
Gantt-Diagramm.

---

## ADR-004: Nur Bootstrap CSS. Keine Bootstrap-JS-Komponenten

**Kontext.** Bootstrap wird mit JS ausgeliefert: Modals, Dropdowns, Offcanvas, Toasts.

**Entscheidung.** Es wird ausschließlich **`bootstrap.min.css`** eingebunden.
`bootstrap.bundle.js` wird nicht eingebunden. Interaktivität wird in Vue implementiert,
unter Verwendung der Bootstrap-CSS-Klassen für das Erscheinungsbild.

**Warum.** Bootstrap JS manipuliert das DOM direkt — hängt Klassen an, fügt Backdrops ein,
verschiebt den Fokus. Vue betrachtet das DOM als sein Eigentum. Zwei Herren über dasselbe
DOM bedeuten schwer reproduzierbare Bugs, die stundenlang debuggt werden müssen. Klassiker:
Vue rendert eine Liste neu, Bootstrap hält eine Referenz auf einen bereits entfernten Knoten.

**Wie das aussieht.** Modal und Drawer — via `<Teleport to="body">` + Bootstrap-CSS-Klassen
(`.modal`, `.offcanvas`, `.show`). Etwa 20 Zeilen pro Komponente, volle Kontrolle, null Konflikte.

**Alternativen und warum nicht.**

- *BootstrapVueNext* (Bootstrap 5-Wrapper für Vue 3) — würde die Aufgabe lösen, ist aber
  eine weitere Abhängigkeit in aktivem Entwicklungsstadium. Widerspricht ADR-005 im Geiste.
- *Bootstrap JS via `ref` + `onMounted`* — funktioniert, erfordert aber manuelle
  Synchronisierung zweier Wahrheitsquellen über den Modal-Zustand.

**Konsequenzen.** Modal, Offcanvas und Toast werden selbst implementiert. Das sind ~60 Zeilen
insgesamt und vollständig vorhersehbares Verhalten.

---

## ADR-005: Eigener HTTP-Client statt axios

**Kontext.** Am 31. März 2026 wurde der npm-Account des axios-Maintainers (`jasonsaayman`)
kompromittiert. Die Versionen **1.14.1** und **0.30.4** enthielten eine gefälschte Abhängigkeit
`plain-crypto-js@4.2.1`, die über `postinstall` einen plattformübergreifenden RAT installierte
und Credentials, SSH-Schlüssel sowie Cloud-Token vom Entwicklerrechner sammelte. Zuschreibung:
Sapphire Sleet (Nordkorea). Sichere Versionen: 1.14.0 / 0.30.3.

**Entscheidung.** axios wird nicht verwendet. Ein eigener Client wird auf Basis von `fetch`
geschrieben — `src/api/client.ts`.

**Warum — und das muss präzise formuliert werden.**

Der Vorfall war ein **Anlass zur Überprüfung, nicht ihre Begründung**. „Wir nehmen kein axios,
weil es gehackt wurde" ist ein schwaches Argument: Der Angriff betraf nicht axios als solches,
sondern die Übernahme eines Maintainer-Accounts plus `postinstall`-Skript. Derselbe
Angriffsvektor steht für **jedes** npm-Paket offen, einschließlich Vue, Vite, Pinia, Bootstrap
und Hunderte transitiver Abhängigkeiten. Wenn die Logik lautet „wir meiden kompromittierte
Pakete" — dann müsste man npm insgesamt meiden. Axios ist heute gepatcht und wurde sorgfältiger
bereinigt als der Durchschnitt.

Der eigentliche Grund ist einfacher: **axios rechnet sich nicht**. `fetch` ist nativ, und axios
darüber hinaus gibt uns nichts von dem, was wir brauchen:

| axios-Funktion | Benötigt? | Wie abgedeckt |
|---|---|---|
| baseURL | ja | 3 Zeilen |
| Auto-JSON | ja | 2 Zeilen |
| Wirft bei 4xx/5xx | ja | 3 Zeilen — der wichtigste Unterschied zu `fetch` |
| Timeout | ja | `AbortSignal.timeout()`, nativ |
| Request-Abbruch | ja | `AbortSignal`, kostenlos |
| Interceptors | **nein** | kein Auth — nichts abzufangen |
| XSRF | **nein** | keine Cookie-Sessions |
| Upload-Fortschritt | **nein** | keine Datei-Uploads. Das Einzige, was `fetch` wirklich nicht kann — dafür bräuchte man XHR |
| Eine API für Node und Browser | **nein** | nur Browser |

Fünf von neun, allesamt trivial. Dazu ist unser Client auf unser OpenAPI typisiert —
was axios out of the box nicht liefert.

**Implementierung vollständig (~65 Zeilen):**

```ts
// src/api/client.ts

/** Ошибка HTTP-уровня. Несёт статус, чтобы вызывающий различал 409 и 422 (MVP §7). */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly body: unknown,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

type Query = Record<string, string | number | boolean | undefined | null>

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PATCH' | 'DELETE'
  body?: unknown
  query?: Query
  signal?: AbortSignal
  timeoutMs?: number
}

const BASE = import.meta.env.VITE_API_BASE ?? '/api/v1'

function buildUrl(path: string, query?: Query): string {
  if (!query) return BASE + path
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== null) params.set(key, String(value))
  }
  const qs = params.toString()
  return qs ? `${BASE}${path}?${qs}` : BASE + path
}

async function safeJson(res: Response): Promise<unknown> {
  try {
    return await res.json()
  } catch {
    return null
  }
}

export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, query, signal, timeoutMs = 10_000 } = opts

  // У fetch нет таймаута. AbortSignal.timeout() нативный — таймер руками не нужен.
  const signals = [AbortSignal.timeout(timeoutMs)]
  if (signal) signals.push(signal)

  const res = await fetch(buildUrl(path, query), {
    method,
    signal: AbortSignal.any(signals),
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })

  // Главное отличие от axios: fetch НЕ бросает на 4xx/5xx.
  if (!res.ok) {
    throw new ApiError(res.status, await safeJson(res), `${method} ${path} → ${res.status}`)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const api = {
  get: <T>(path: string, o?: Omit<RequestOptions, 'method' | 'body'>) =>
    request<T>(path, { ...o, method: 'GET' }),
  post: <T>(path: string, body?: unknown, o?: Omit<RequestOptions, 'method' | 'body'>) =>
    request<T>(path, { ...o, method: 'POST', body }),
  patch: <T>(path: string, body?: unknown, o?: Omit<RequestOptions, 'method' | 'body'>) =>
    request<T>(path, { ...o, method: 'PATCH', body }),
  delete: <T>(path: string, o?: Omit<RequestOptions, 'method' | 'body'>) =>
    request<T>(path, { ...o, method: 'DELETE' }),
}
```

**Verwendung — direkt auf unsere Antwortcodes zugeschnitten:**

```ts
import { api, ApiError } from '@/api/client'
import type { Buchung } from '@/api/types'

const buchungen = await api.get<Buchung[]>('/buchungen', {
  query: { standort_id: 1, datum: '2026-07-20' },
})

try {
  await api.post<Buchung>('/buchungen', payload)
} catch (e) {
  if (e instanceof ApiError && e.status === 409) {
    toast('Slot wurde gerade vergeben')   // слот занят — попробуй другой
  } else if (e instanceof ApiError && e.status === 422) {
    toast(`Ungültig: ${e.body}`)          // нарушено доменное правило
  }
}
```

**Was wirklich vor dieser Klasse von Angriffen schützt** (im Gegensatz zu „wir nehmen kein axios"):

- `npm ci` mit eingecheckem `package-lock.json` — nicht `npm install`
- Exakte Versionen ohne `^` und `~`
- **`npm config set ignore-scripts true`** — genau `postinstall` war der Angriffsvektor
- Regelmäßiges `npm audit`, Dependabot
- Minimale Abhängigkeiten — der einzige Schutz, der skaliert

**Konsequenzen.** Keine Interceptors — kommt Auth hinzu, wird der Header in `request()` in einer
Zeile ergänzt. Kein Upload-Fortschritt — nicht benötigt. `AbortSignal.any()` erfordert einen
modernen Browser (Chrome 116+, Safari 17.4+, Firefox 124+) — für unseren Anwendungsfall
akzeptabel.

---

## ADR-006: Pinia für den Zustand

**Entscheidung.** Pinia. Zwei Stores: `stammdaten.ts`, `buchungen.ts`.

**Warum.** `buchungen.ts` ist die **Echtzeit-Quelle** in der Demo-Konsole. Alle drei Panels
lesen denselben Store; eine Buchung mutiert den Store → Vues Reaktivität aktualisiert die
anderen Panels sofort. Weder SSE noch WebSockets werden benötigt (MVP §8).

**Konsequenzen.** Echtzeit funktioniert nur innerhalb eines Browser-Tabs. Zwei Tabs werden
nicht synchronisiert — für die Demo-Konsole irrelevant, für ein Produkt wäre SSE erforderlich
(MVP §10.I).

---

## ADR-007: Kein Router

**Entscheidung.** `vue-router` wird nicht eingebunden.

**Warum.** Die Konsole ist einseitig: Alle drei Panels sind gleichzeitig sichtbar, Stammdaten
befinden sich in einem Drawer. Es gibt keine Rollenauswahl (MVP §4). Es gibt nichts zu routen.

**Konsequenzen.** Werden separate Rollen-Bildschirme hinzugefügt (MVP §10.E), wird ein Router
ergänzt. Derzeit wäre er eine Abhängigkeit ohne Aufgabe.

---

## ADR-008: Domänenbegriffe auf Deutsch

**Entscheidung.** Wie im Backend (§4 CONVENTIONS): `Buchung`, `Rampe`, `Standort`,
`Temperaturbereich`, `Kennzeichen`. Komponenten heißen `FahrerPanel.vue`, `TorListe.vue`.

**Warum.** Eine einheitliche Domänensprache von der SQLite-Tabelle bis zur UI-Überschrift.
Keine Übersetzungen an den Grenzen — keine Abweichungen.

---

## Abhängigkeiten: Zusammenfassung

```
vue            ← ADR-001
typescript     ← ADR-002
vite
pinia          ← ADR-006
bootstrap      ← ADR-003 (nur CSS, ADR-004)
```

Das ist alles. Kein axios, kein vue-router, keine UI-Wrapper, kein Tailwind. Jede Abhängigkeit
wird durch einen einzigen ADR erklärt — lässt sich keine Erklärung finden, kommt sie nicht rein.

---

## Anhang: React → Vue 3

Umstiegs-Spickzettel. Konzepte, kein Syntax.

| React | Vue 3 (`<script setup>`) |
|---|---|
| `useState` | `ref()` (Primitive) / `reactive()` (Objekte) |
| `useMemo` | `computed()` |
| `useEffect(fn, [deps])` | `watch(deps, fn)` |
| `useEffect(fn, [])` | `onMounted(fn)` |
| `useEffect` mit Cleanup | `onUnmounted(fn)` / Rückgabewert aus `watchEffect` |
| `useCallback` | nicht nötig — Funktionen werden nicht neu erstellt |
| `React.memo` | nicht nötig — Reaktivität ist granular |
| `useRef` (DOM) | `ref()` + `ref="name"` im Template |
| `useContext` | `provide()` / `inject()` |
| Redux / Zustand | Pinia |
| JSX | `<template>` |
| `className` | `class` |
| `{cond && <X/>}` | `<X v-if="cond"/>` |
| `{items.map(...)}` | `<li v-for="i in items" :key="i.id">` |
| `value` + `onChange` | `v-model` |
| props | `defineProps<{...}>()` |
| Callback-Props (`onSave`) | `defineEmits<{save: [id: number]}>()` |
| `children` | `<slot/>` |
| Portal | `<Teleport to="body">` |

### Drei Dinge, über die alle React-Entwickler stolpern

**1. `.value` im Script, aber nicht im Template.**

```ts
const count = ref(0)
count.value++              // im <script> — über .value
```
```html
<span>{{ count }}</span>   <!-- im <template> — wird automatisch aufgelöst -->
```

Das nervt die erste Woche, danach bemerkt man es nicht mehr.

**2. Mutieren — erlaubt und erwünscht.**

```ts
const store = useBuchungenStore()
store.buchungen.push(neue)     // so ist es richtig
```

Kein `setState([...prev, x])`, keine Immutabilität. Vue verfolgt Mutationen via Proxy.
Der React-Reflex „ich mache eine Kopie" schadet hier nur.

**3. Abhängigkeiten werden nicht deklariert.**

```ts
const freieSlots = computed(() =>
  alleSlots.value.filter(s => !belegte.value.has(s.id))
)
```

Kein Dependency-Array, nichts zu vergessen. `computed` weiß selbst, was es gelesen hat.
Es wird genau dann neu berechnet, wenn nötig, und gecacht, solange es nicht nötig ist.

### Vollständiges Komponenten-Gerüst

```vue
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api, ApiError } from '@/api/client'
import type { Buchung } from '@/api/types'

const props = defineProps<{ standortId: number; datum: string }>()
const emit = defineEmits<{ gebucht: [b: Buchung] }>()

const buchungen = ref<Buchung[]>([])
const laedt = ref(false)
const fehler = ref<string | null>(null)

const anzahlTK = computed(
  () => buchungen.value.filter(b => b.temperaturbereich === 'TK').length
)

async function laden() {
  laedt.value = true
  fehler.value = null
  try {
    buchungen.value = await api.get<Buchung[]>('/buchungen', {
      query: { standort_id: props.standortId, datum: props.datum },
    })
  } catch (e) {
    fehler.value = e instanceof ApiError ? `Fehler ${e.status}` : 'Netzwerkfehler'
  } finally {
    laedt.value = false
  }
}

onMounted(laden)
</script>

<template>
  <div class="card">
    <div class="card-header d-flex justify-content-between">
      <span>Buchungen</span>
      <span class="badge text-bg-primary">{{ anzahlTK }} TK</span>
    </div>

    <div v-if="laedt" class="card-body text-muted">Lädt…</div>
    <div v-else-if="fehler" class="alert alert-danger m-3">{{ fehler }}</div>
    <ul v-else class="list-group list-group-flush">
      <li v-for="b in buchungen" :key="b.id" class="list-group-item">
        {{ b.beginn }} · {{ b.kennzeichen }} · {{ b.spediteur }}
      </li>
    </ul>
  </div>
</template>
```

Alles, was in `<script setup>` deklariert wird, steht im `<template>` automatisch zur
Verfügung — kein Export, kein `return`. Die Klassen im Template sind Bootstrap-Klassen (ADR-003).

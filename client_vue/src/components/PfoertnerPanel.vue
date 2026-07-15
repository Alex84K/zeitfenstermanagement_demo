<script setup lang="ts">
import { ref, computed } from 'vue'
import { useBuchungenStore } from '@/stores/buchungen'
import { useStammdatenStore } from '@/stores/stammdaten'
import { ApiError } from '@/api/client'
import type { Buchung, EreignisTyp, BuchungStatus } from '@/api/types'

const buchungenStore = useBuchungenStore()
const stammdaten = useStammdatenStore()

const suche = ref('')
const fehler = ref<string | null>(null)

const tz = computed(() => stammdaten.activeStandort?.zeitzone ?? 'Europe/Berlin')

// Buchungen gefiltert nach Kennzeichen-Suche
const gefilterteBuchungen = computed(() => {
  const q = suche.value.trim().toLowerCase()
  if (!q) return buchungenStore.buchungen
  return buchungenStore.buchungen.filter(
    (b) =>
      b.kennzeichen.toLowerCase().includes(q) ||
      b.spediteur.toLowerCase().includes(q),
  )
})

function formatZeit(iso: string): string {
  return new Date(iso).toLocaleTimeString('de-DE', {
    hour: '2-digit', minute: '2-digit', timeZone: tz.value,
  })
}

// Sektionen: Erwartet, Auf Gelände, No-Shows/Verspätet, Abgefahren
const erwartet = computed(() =>
  gefilterteBuchungen.value.filter((b) => b.status === 'GEBUCHT'),
)
const aufGelaende = computed(() =>
  gefilterteBuchungen.value.filter(
    (b) => b.status === 'ANGEKOMMEN' || b.status === 'ANGEDOCKT',
  ),
)
const noShows = computed(() =>
  gefilterteBuchungen.value.filter((b) => b.status === 'NO_SHOW'),
)
const abgefahren = computed(() =>
  gefilterteBuchungen.value.filter((b) => b.status === 'ABGEFAHREN'),
)

// Erlaubte nächste Ereignis-Typen je Status
function naechsteEreignisse(status: BuchungStatus): EreignisTyp[] {
  if (status === 'GEBUCHT') return ['ANGEKOMMEN']
  if (status === 'ANGEKOMMEN') return ['ANGEDOCKT']
  if (status === 'ANGEDOCKT') return ['ABGEFAHREN']
  return []
}

const EREIGNIS_LABEL: Record<EreignisTyp, string> = {
  ANGEKOMMEN: 'Ankunft',
  ANGEDOCKT: 'Andockung',
  ABGEFAHREN: 'Abfahrt',
}

async function ereignisErfassen(b: Buchung, typ: EreignisTyp) {
  fehler.value = null
  try {
    await buchungenStore.erfasseEreignis(b.id, typ)
  } catch (e) {
    if (e instanceof ApiError) {
      const body = e.body as { message?: string } | null
      fehler.value = body?.message ?? e.message
    } else {
      fehler.value = 'Netzwerkfehler'
    }
  }
}

const STATUS_BADGE: Record<BuchungStatus, string> = {
  GEBUCHT: 'secondary', ANGEKOMMEN: 'info', ANGEDOCKT: 'primary',
  ABGEFAHREN: 'success', NO_SHOW: 'danger', STORNIERT: 'dark',
}
</script>

<template>
  <div class="d-flex flex-column h-100 overflow-y-auto p-2 gap-2" style="font-size:0.83rem">
    <div class="fw-semibold text-uppercase text-secondary" style="font-size:0.7rem;letter-spacing:.05em">
      Pförtner
    </div>

    <!-- Kennzeichen-Suche -->
    <input
      v-model="suche"
      type="text"
      class="form-control form-control-sm"
      placeholder="Kennzeichen suchen…"
    />

    <div v-if="fehler" class="alert alert-danger p-2 small">{{ fehler }}</div>

    <!-- Erwartet -->
    <template v-if="erwartet.length > 0">
      <div class="fw-semibold small text-secondary">Erwartet</div>
      <div v-for="b in erwartet" :key="b.id" class="border rounded p-2 d-flex flex-column gap-1">
        <div class="d-flex align-items-center gap-2">
          <strong>{{ formatZeit(b.beginn) }}</strong>
          <span class="fw-semibold">{{ b.kennzeichen }}</span>
          <span class="badge" :class="`text-bg-${STATUS_BADGE[b.status]}`">{{ b.temperaturbereich }}</span>
        </div>
        <div class="text-muted small">{{ b.spediteur }}</div>
        <div class="d-flex gap-1 flex-wrap">
          <button
            v-for="typ in naechsteEreignisse(b.status)"
            :key="typ"
            class="btn btn-sm btn-outline-primary"
            @click="ereignisErfassen(b, typ)"
          >
            {{ EREIGNIS_LABEL[typ] }}
          </button>
        </div>
      </div>
    </template>

    <!-- Auf Gelände -->
    <template v-if="aufGelaende.length > 0">
      <div class="fw-semibold small text-secondary">Auf Gelände</div>
      <div v-for="b in aufGelaende" :key="b.id" class="border rounded p-2 d-flex flex-column gap-1">
        <div class="d-flex align-items-center gap-2">
          <strong>{{ formatZeit(b.beginn) }}</strong>
          <span class="fw-semibold">{{ b.kennzeichen }}</span>
          <span class="badge" :class="`text-bg-${STATUS_BADGE[b.status]}`">{{ b.status }}</span>
        </div>
        <div class="text-muted small">{{ b.spediteur }}</div>
        <div class="d-flex gap-1 flex-wrap">
          <button
            v-for="typ in naechsteEreignisse(b.status)"
            :key="typ"
            class="btn btn-sm btn-outline-primary"
            @click="ereignisErfassen(b, typ)"
          >
            {{ EREIGNIS_LABEL[typ] }}
          </button>
        </div>
      </div>
    </template>

    <!-- No-Shows -->
    <template v-if="noShows.length > 0">
      <div class="fw-semibold small text-danger">No-Shows</div>
      <div v-for="b in noShows" :key="b.id" class="border border-danger rounded p-2">
        <div class="d-flex align-items-center gap-2">
          <strong>{{ formatZeit(b.beginn) }}</strong>
          <span class="fw-semibold">{{ b.kennzeichen }}</span>
          <span class="text-danger">⚠</span>
        </div>
        <div class="text-muted small">{{ b.spediteur }}</div>
      </div>
    </template>

    <!-- Abgefahren -->
    <template v-if="abgefahren.length > 0">
      <div class="fw-semibold small text-muted">Abgefahren</div>
      <div v-for="b in abgefahren" :key="b.id" class="border rounded p-2 opacity-75">
        <div class="d-flex align-items-center gap-2">
          <span>{{ formatZeit(b.beginn) }}</span>
          <span class="fw-semibold">{{ b.kennzeichen }}</span>
        </div>
        <div class="text-muted small">{{ b.spediteur }}</div>
      </div>
    </template>

    <div v-if="gefilterteBuchungen.length === 0" class="text-muted small text-center py-3">
      Keine Buchungen
    </div>
  </div>
</template>

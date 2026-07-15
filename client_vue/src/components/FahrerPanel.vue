<script setup lang="ts">
import { ref, computed } from 'vue'
import { useBuchungenStore } from '@/stores/buchungen'
import { useStammdatenStore } from '@/stores/stammdaten'
import { ApiError } from '@/api/client'
import type { Temperaturbereich, VerfuegbarkeitsSlot, Buchung } from '@/api/types'

const buchungen = useBuchungenStore()
const stammdaten = useStammdatenStore()

const temperaturbereich = ref<Temperaturbereich>('TK')
const dauerMin = ref(60)
const spediteur = ref('')
const kennzeichen = ref('')

const suchelaeuft = ref(false)
const freieSlots = ref<VerfuegbarkeitsSlot[]>([])
const meineBuchungen = ref<Buchung[]>([])
const fehler = ref<string | null>(null)

const tz = computed(() => stammdaten.activeStandort?.zeitzone ?? 'Europe/Berlin')

async function suchen() {
  suchelaeuft.value = true
  fehler.value = null
  freieSlots.value = []
  try {
    freieSlots.value = await buchungen.sucheVerfuegbarkeit(temperaturbereich.value, dauerMin.value)
    if (spediteur.value.trim()) {
      meineBuchungen.value = await buchungen.sucheMeineBuchungen(spediteur.value.trim())
    }
  } catch (e) {
    fehler.value = e instanceof ApiError ? e.message : 'Netzwerkfehler'
  } finally {
    suchelaeuft.value = false
  }
}

async function buchen(slot: VerfuegbarkeitsSlot) {
  if (!kennzeichen.value.trim()) { fehler.value = 'Kennzeichen eingeben'; return }
  if (!spediteur.value.trim()) { fehler.value = 'Spediteur eingeben'; return }
  fehler.value = null
  try {
    await buchungen.erstelleBuchung({
      rampe_id: slot.rampe_id,
      spediteur: spediteur.value.trim(),
      kennzeichen: kennzeichen.value.trim(),
      sendung_nr: '',
      temperaturbereich: temperaturbereich.value,
      beginn: slot.beginn,
      ende: slot.ende,
    })
    await suchen()
  } catch (e) {
    if (e instanceof ApiError) {
      const body = e.body as { message?: string } | null
      fehler.value = body?.message ?? e.message
    } else {
      fehler.value = 'Netzwerkfehler'
    }
  }
}

async function stornieren(id: number) {
  try {
    await buchungen.stornieren(id)
    meineBuchungen.value = meineBuchungen.value.filter((b) => b.id !== id)
  } catch (e) {
    fehler.value = e instanceof ApiError ? e.message : 'Netzwerkfehler'
  }
}

function formatZeit(iso: string): string {
  return new Date(iso).toLocaleTimeString('de-DE', {
    hour: '2-digit', minute: '2-digit', timeZone: tz.value,
  })
}

const STATUS_LABEL: Record<string, string> = {
  GEBUCHT: 'Gebucht', ANGEKOMMEN: 'Angekommen', ANGEDOCKT: 'Angedockt',
  ABGEFAHREN: 'Abgefahren', NO_SHOW: 'No-Show', STORNIERT: 'Storniert',
}
</script>

<template>
  <div class="d-flex flex-column h-100 overflow-y-auto p-2 gap-2" style="font-size:0.85rem">
    <div class="fw-semibold text-uppercase text-secondary" style="font-size:0.7rem;letter-spacing:.05em">
      Fahrer
    </div>

    <!-- Suchmaske -->
    <div class="card card-body p-2 gap-2 d-flex flex-column">
      <div>
        <label class="form-label mb-1">Datum</label>
        <input
          type="date"
          class="form-control form-control-sm"
          :value="buchungen.datum"
          @change="buchungen.setDatum(($event.target as HTMLInputElement).value)"
        />
      </div>
      <div>
        <label class="form-label mb-1">Temperatur</label>
        <select v-model="temperaturbereich" class="form-select form-select-sm">
          <option value="TK">TK – Tiefkühl</option>
          <option value="FRISCH">Frisch</option>
          <option value="TROCKEN">Trocken</option>
        </select>
      </div>
      <div>
        <label class="form-label mb-1">Dauer</label>
        <select v-model="dauerMin" class="form-select form-select-sm">
          <option :value="30">30 Min</option>
          <option :value="60">1 Std</option>
          <option :value="90">1,5 Std</option>
          <option :value="120">2 Std</option>
          <option :value="150">2,5 Std</option>
          <option :value="180">3 Std</option>
          <option :value="240">4 Std</option>
        </select>
      </div>
      <div>
        <label class="form-label mb-1">Kennzeichen</label>
        <input
          v-model="kennzeichen"
          type="text"
          class="form-control form-control-sm"
          placeholder="GT-ML 1234"
          @input="fehler = null"
        />
      </div>
      <div>
        <label class="form-label mb-1">Spediteur</label>
        <input
          v-model="spediteur"
          type="text"
          class="form-control form-control-sm"
          placeholder="Spedition Müller GmbH"
          @input="fehler = null"
        />
      </div>
      <button class="btn btn-primary btn-sm" :disabled="suchelaeuft" @click="suchen">
        {{ suchelaeuft ? 'Suche…' : 'Suchen' }}
      </button>
    </div>

    <div v-if="fehler" class="alert alert-danger p-2 small">{{ fehler }}</div>

    <!-- Freie Fenster -->
    <template v-if="freieSlots.length > 0">
      <div class="fw-semibold small">Freie Fenster</div>
      <div
        v-for="slot in freieSlots"
        :key="`${slot.rampe_id}-${slot.beginn}`"
        class="d-flex align-items-center justify-content-between border rounded p-2"
      >
        <span>
          <strong>{{ formatZeit(slot.beginn) }}</strong>
          <span class="text-muted"> {{ slot.bezeichnung }}</span>
        </span>
        <button class="btn btn-sm btn-outline-primary" @click="buchen(slot)">Buchen</button>
      </div>
    </template>
    <div v-else-if="!suchelaeuft && freieSlots.length === 0 && buchungen.buchungen.length > 0" class="text-muted small">
      Keine freien Fenster
    </div>

    <!-- Meine Buchungen -->
    <template v-if="meineBuchungen.length > 0">
      <div class="fw-semibold small mt-1">Meine Buchungen</div>
      <div
        v-for="b in meineBuchungen"
        :key="b.id"
        class="border rounded p-2 d-flex flex-column gap-1"
      >
        <div class="d-flex justify-content-between">
          <span>
            <strong>{{ formatZeit(b.beginn) }}</strong>
            <span class="text-muted"> R.{{ b.rampe_id }}</span>
          </span>
          <span class="badge text-bg-secondary">{{ STATUS_LABEL[b.status] }}</span>
        </div>
        <div class="text-muted">{{ b.temperaturbereich }} · {{ b.kennzeichen }}</div>
        <button
          v-if="b.status === 'GEBUCHT'"
          class="btn btn-sm btn-outline-danger"
          @click="stornieren(b.id)"
        >
          Stornieren
        </button>
      </div>
    </template>
  </div>
</template>

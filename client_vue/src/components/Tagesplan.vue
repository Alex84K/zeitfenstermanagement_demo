<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useStammdatenStore } from '@/stores/stammdaten'
import { useBuchungenStore } from '@/stores/buchungen'
import type { Buchung, Rampe, Temperaturbereich, BuchungStatus } from '@/api/types'

const emit = defineEmits<{ slotKlick: [rampe: Rampe, beginnISO: string] }>()

const stammdaten = useStammdatenStore()
const buchungen = useBuchungenStore()

const standort = computed(() => stammdaten.activeStandort)
const tz = computed(() => standort.value?.zeitzone ?? 'Europe/Berlin')
const rampen = computed(() => stammdaten.rampen.filter((r) => r.aktiv))

function parseHHMM(s: string): number {
  const [h, m] = s.split(':').map(Number)
  return (h ?? 0) * 60 + (m ?? 0)
}

const oeffVon = computed(() => parseHHMM(standort.value?.oeffnung_von ?? '05:00'))
const oeffBis = computed(() => parseHHMM(standort.value?.oeffnung_bis ?? '22:00'))
const totalSlots = computed(() => (oeffBis.value - oeffVon.value) / 30)

// Header-Uhrzeiten: jede volle Stunde
const headerUhrzeiten = computed(() => {
  const result: string[] = []
  for (let min = oeffVon.value; min <= oeffBis.value; min += 60) {
    result.push(`${Math.floor(min / 60).toString().padStart(2, '0')}`)
  }
  return result
})

function toLocalMinutes(iso: string): number {
  const date = new Date(iso)
  const parts = new Intl.DateTimeFormat('de-DE', {
    hour: '2-digit', minute: '2-digit', timeZone: tz.value, hour12: false,
  }).formatToParts(date)
  const h = parseInt(parts.find((p) => p.type === 'hour')?.value ?? '0')
  const m = parseInt(parts.find((p) => p.type === 'minute')?.value ?? '0')
  return h * 60 + m
}

function buchungStyle(b: Buchung) {
  const startMin = toLocalMinutes(b.beginn) - oeffVon.value
  const endMin = toLocalMinutes(b.ende) - oeffVon.value
  const startSlot = startMin / 30
  const spanSlots = (endMin - startMin) / 30

  const color = TB_COLOR[b.temperaturbereich]
  const styles = STATUS_STYLE[b.status]

  return {
    gridColumn: `${Math.round(startSlot) + 2} / span ${Math.max(1, Math.round(spanSlots))}`,
    backgroundColor: styles.bg ?? color,
    opacity: styles.opacity,
    border: styles.border ? `2px ${styles.borderStyle ?? 'solid'} ${color}` : 'none',
    borderRadius: '3px',
    color: '#fff',
    fontSize: '0.7rem',
    overflow: 'hidden',
    cursor: 'default',
    zIndex: 2,
    alignSelf: 'stretch',
    margin: '1px 0',
    padding: '1px 3px',
    display: 'flex',
    alignItems: 'center',
  }
}

const TB_COLOR: Record<Temperaturbereich, string> = {
  TK: '#1565C0',
  FRISCH: '#00695C',
  TROCKEN: '#546E7A',
}

interface StatusStyle {
  opacity: number
  border: boolean
  borderStyle?: string
  bg?: string
}

const STATUS_STYLE: Record<BuchungStatus, StatusStyle> = {
  GEBUCHT:    { opacity: 0.45, border: true, borderStyle: 'dashed' },
  ANGEKOMMEN: { opacity: 0.70, border: true },
  ANGEDOCKT:  { opacity: 1.0,  border: true },
  ABGEFAHREN: { opacity: 0.35, border: false },
  NO_SHOW:    { opacity: 1.0,  border: false, bg: '#B71C1C' },
  STORNIERT:  { opacity: 0.4,  border: false, bg: '#9E9E9E' },
}

function buchungenFuerRampe(rampeId: number): Buchung[] {
  return buchungen.buchungen.filter((b) => b.rampe_id === rampeId)
}

// "Jetzt"-Linie
const jetztFraction = ref(0)

function aktualisiereJetzt() {
  const now = new Date()
  const parts = new Intl.DateTimeFormat('de-DE', {
    hour: '2-digit', minute: '2-digit',
    timeZone: tz.value, hour12: false,
  }).formatToParts(now)
  const h = parseInt(parts.find((p) => p.type === 'hour')?.value ?? '0')
  const m = parseInt(parts.find((p) => p.type === 'minute')?.value ?? '0')
  const nowMin = h * 60 + m
  jetztFraction.value = Math.max(0, Math.min(1, (nowMin - oeffVon.value) / (oeffBis.value - oeffVon.value)))
}

let timer: ReturnType<typeof setInterval>
onMounted(() => {
  aktualisiereJetzt()
  timer = setInterval(aktualisiereJetzt, 30_000)
})
onUnmounted(() => clearInterval(timer))

// Klick auf leere Rasterzelle → Buchungsdialog
function onRowClick(e: MouseEvent, rampe: Rampe) {
  const row = e.currentTarget as HTMLElement
  const rect = row.getBoundingClientRect()
  const labelWidth = 80
  const clickX = e.clientX - rect.left - labelWidth
  if (clickX < 0) return

  const slotsAreaWidth = rect.width - labelWidth
  const slotIndex = Math.floor((clickX / slotsAreaWidth) * totalSlots.value)
  const slotMin = oeffVon.value + slotIndex * 30

  // Convert local slot time on current datum to UTC ISO
  const d = buchungen.datum // YYYY-MM-DD
  const localDateTimeStr = `${d}T${String(Math.floor(slotMin / 60)).padStart(2, '0')}:${String(slotMin % 60).padStart(2, '0')}:00`

  // Build as if in the Standort timezone
  const utcDate = localToUTC(localDateTimeStr, tz.value)
  emit('slotKlick', rampe, utcDate.toISOString())
}

// Converts a naive datetime string (YYYY-MM-DDTHH:MM:SS) in given timezone to UTC Date.
// We do this by formatting a known UTC time and binary-searching until it matches.
// Simpler: use the offset approach via Intl.
function localToUTC(localStr: string, timezone: string): Date {
  // Use the trick: create a date in UTC, then format it in the target timezone,
  // compute the offset, and subtract it.
  const naive = new Date(localStr + 'Z') // pretend UTC
  const naiveParts = new Intl.DateTimeFormat('de-DE', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
    timeZone: timezone, hour12: false,
  }).formatToParts(naive)

  const get = (type: string) => parseInt(naiveParts.find((p) => p.type === type)?.value ?? '0')
  const localMs = Date.UTC(get('year'), get('month') - 1, get('day'), get('hour'), get('minute'), get('second'))
  const offsetMs = naive.getTime() - localMs
  return new Date(naive.getTime() + offsetMs)
}

function statusLabel(s: BuchungStatus): string {
  const MAP: Record<BuchungStatus, string> = {
    GEBUCHT: 'G', ANGEKOMMEN: 'A', ANGEDOCKT: 'D', ABGEFAHREN: 'F', NO_SHOW: '✗', STORNIERT: '–',
  }
  return MAP[s]
}

const LABEL_WIDTH = 80
</script>

<template>
  <div class="position-relative" style="overflow-x:auto;overflow-y:hidden;height:100%">
    <!-- Kein Standort -->
    <div v-if="!standort" class="d-flex align-items-center justify-content-center h-100 text-muted">
      Kein Standort ausgewählt
    </div>

    <div v-else class="d-flex flex-column h-100" style="min-width:600px">
      <!-- Zeitachse-Header -->
      <div
        class="d-grid flex-shrink-0 border-bottom"
        :style="{
          gridTemplateColumns: `${LABEL_WIDTH}px repeat(${totalSlots}, minmax(20px, 1fr))`,
          display: 'grid',
        }"
      >
        <div class="text-muted" style="font-size:0.7rem;padding:2px 4px" />
        <!-- Eine Spalte pro 30-Min-Slot, Label nur bei vollen Stunden -->
        <div
          v-for="i in totalSlots"
          :key="i"
          class="text-muted text-center border-start"
          style="font-size:0.65rem;padding:1px 0"
        >
          <template v-if="(i - 1) % 2 === 0">
            {{ headerUhrzeiten[(i - 1) / 2] }}
          </template>
        </div>
      </div>

      <!-- Rampen-Zeilen mit "Jetzt"-Linie -->
      <div class="position-relative flex-grow-1 overflow-y-auto">
        <!-- "Jetzt"-Linie -->
        <div
          class="position-absolute top-0 bottom-0"
          style="width:2px;background:#dc3545;z-index:5;pointer-events:none"
          :style="{ left: `calc(${LABEL_WIDTH}px + (100% - ${LABEL_WIDTH}px) * ${jetztFraction})` }"
        />

        <!-- Rampe-Zeile -->
        <div
          v-for="rampe in rampen"
          :key="rampe.id"
          class="d-grid border-bottom position-relative"
          style="min-height:36px;cursor:crosshair"
          :style="{
            gridTemplateColumns: `${LABEL_WIDTH}px repeat(${totalSlots}, minmax(20px, 1fr))`,
            display: 'grid',
          }"
          @click="onRowClick($event, rampe)"
        >
          <!-- Rampe-Label -->
          <div
            class="d-flex align-items-center px-1 border-end bg-light"
            style="font-size:0.75rem;font-weight:500;z-index:1;position:sticky;left:0"
          >
            {{ rampe.bezeichnung }}
          </div>

          <!-- Hintergrund-Slots (alternierend) -->
          <div
            v-for="i in totalSlots"
            :key="i"
            class="border-start"
            :class="(i - 1) % 2 === 0 ? 'bg-white' : ''"
            style="background-color: rgba(0,0,0,0.02)"
          />

          <!-- Buchungs-Blöcke (absolut positioniert über dem Grid) -->
          <div
            v-for="b in buchungenFuerRampe(rampe.id)"
            :key="b.id"
            :style="buchungStyle(b)"
            :title="`${b.kennzeichen} · ${b.spediteur} · ${b.status}`"
          >
            {{ statusLabel(b.status) }} {{ b.kennzeichen }}
          </div>
        </div>

        <div v-if="rampen.length === 0" class="text-center text-muted py-4 small">
          Keine aktiven Rampen
        </div>
      </div>
    </div>
  </div>
</template>

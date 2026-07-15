<script setup lang="ts">
import { computed } from 'vue'
import { useBuchungenStore } from '@/stores/buchungen'

const store = useBuchungenStore()
const kz = computed(() => store.kennzahlen)

function fmt(v: number | null, suffix: string): string {
  if (v === null) return '–'
  return `${Math.round(v)} ${suffix}`
}
</script>

<template>
  <div class="bg-light border-bottom px-3 py-1 d-flex gap-4 align-items-center flex-shrink-0 small">
    <span>
      <span class="text-muted">Ø Standzeit </span>
      <strong>{{ fmt(kz?.standzeit_schnitt_min ?? null, 'Min') }}</strong>
    </span>
    <span>
      <span class="text-muted">Pünktlichkeit </span>
      <strong>{{ fmt(kz?.puenktlichkeit_prozent ?? null, '%') }}</strong>
    </span>
    <span>
      <span class="text-muted">No-Shows </span>
      <strong :class="kz && kz.no_shows > 0 ? 'text-danger' : ''">
        {{ kz?.no_shows ?? '–' }}
      </strong>
    </span>
    <span class="text-muted ms-auto">
      {{ kz?.buchungen_gesamt ?? '–' }} Buchungen
    </span>
  </div>
</template>

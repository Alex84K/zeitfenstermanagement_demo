<script setup lang="ts">
import Tagesplan from './Tagesplan.vue'
import type { Rampe } from '@/api/types'

const emit = defineEmits<{ slotKlick: [rampe: Rampe, beginnISO: string] }>()
</script>

<template>
  <div class="d-flex flex-column h-100 overflow-hidden">
    <div class="fw-semibold text-uppercase text-secondary px-2 pt-2 flex-shrink-0" style="font-size:0.7rem;letter-spacing:.05em">
      Disponent
    </div>

    <!-- Legende -->
    <div class="d-flex gap-3 px-2 py-1 flex-shrink-0" style="font-size:0.7rem">
      <span class="d-flex align-items-center gap-1">
        <span class="d-inline-block rounded" style="width:12px;height:12px;background:#1565C0;opacity:.9" />TK
      </span>
      <span class="d-flex align-items-center gap-1">
        <span class="d-inline-block rounded" style="width:12px;height:12px;background:#00695C;opacity:.9" />Frisch
      </span>
      <span class="d-flex align-items-center gap-1">
        <span class="d-inline-block rounded" style="width:12px;height:12px;background:#546E7A;opacity:.9" />Trocken
      </span>
      <span class="text-muted ms-2">Status: G=Gebucht A=Angekommen D=Angedockt F=Abgefahren ✗=No-Show</span>
    </div>

    <Tagesplan class="flex-grow-1" @slot-klick="(rampe, beginn) => emit('slotKlick', rampe, beginn)" />
  </div>
</template>

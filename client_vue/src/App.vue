<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useStammdatenStore } from '@/stores/stammdaten'
import { useBuchungenStore } from '@/stores/buchungen'
import KennzahlenLeiste from '@/components/KennzahlenLeiste.vue'
import FahrerPanel from '@/components/FahrerPanel.vue'
import DisponentPanel from '@/components/DisponentPanel.vue'
import PfoertnerPanel from '@/components/PfoertnerPanel.vue'
import StammdatenDrawer from '@/components/StammdatenDrawer.vue'
import BuchungDialog from '@/components/BuchungDialog.vue'
import type { Rampe } from '@/api/types'

const stammdaten = useStammdatenStore()
const buchungen = useBuchungenStore()

const drawerOffen = ref(false)
const dialogOffen = ref(false)
const dialogRampe = ref<Rampe | null>(null)
const dialogBeginn = ref('')

onMounted(async () => {
  await stammdaten.ladeStandorte()
  await buchungen.laden()
})

watch(() => stammdaten.activeStandortId, async (id) => {
  if (id !== null) {
    await stammdaten.ladeRampen(id)
    await buchungen.laden()
  }
})

watch(() => buchungen.datum, () => buchungen.laden())

function onSlotKlick(rampe: Rampe, beginnISO: string) {
  dialogRampe.value = rampe
  dialogBeginn.value = beginnISO
  dialogOffen.value = true
}

async function onBuchungErstellt() {
  dialogOffen.value = false
  await buchungen.laden()
}
</script>

<template>
  <div class="d-flex flex-column vh-100 overflow-hidden">
    <!-- Kopfzeile -->
    <header class="bg-dark text-white px-3 py-2 d-flex align-items-center gap-3 flex-shrink-0">
      <span class="fw-semibold">Zeitfenstermanagement</span>
      <span v-if="stammdaten.activeStandort" class="text-secondary">
        · {{ stammdaten.activeStandort.name }}
      </span>
      <span class="text-secondary">
        · {{ new Date(buchungen.datum).toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit', year: 'numeric' }) }}
      </span>
      <div class="ms-auto d-flex gap-2 align-items-center">
        <select
          v-if="stammdaten.standorte.length > 1"
          class="form-select form-select-sm bg-dark text-white border-secondary"
          style="width:auto"
          :value="stammdaten.activeStandortId"
          @change="stammdaten.waehleStandort(+($event.target as HTMLSelectElement).value)"
        >
          <option v-for="s in stammdaten.standorte" :key="s.id" :value="s.id">{{ s.name }}</option>
        </select>
        <button class="btn btn-sm btn-outline-light" @click="drawerOffen = true">⚙ Stammdaten</button>
      </div>
    </header>

    <!-- KPI-Leiste -->
    <KennzahlenLeiste />

    <!-- Drei Panels -->
    <div class="d-flex flex-row flex-grow-1 overflow-hidden">
      <FahrerPanel class="border-end" style="width:260px;flex-shrink:0" />
      <DisponentPanel class="flex-grow-1 overflow-hidden" @slot-klick="onSlotKlick" />
      <PfoertnerPanel class="border-start" style="width:280px;flex-shrink:0" />
    </div>
  </div>

  <!-- Stammdaten-Drawer -->
  <StammdatenDrawer :open="drawerOffen" @close="drawerOffen = false" />

  <!-- Buchung-Dialog -->
  <BuchungDialog
    v-if="dialogOffen && dialogRampe"
    :rampe="dialogRampe"
    :beginn-iso="dialogBeginn"
    @close="dialogOffen = false"
    @gebucht="onBuchungErstellt"
  />
</template>

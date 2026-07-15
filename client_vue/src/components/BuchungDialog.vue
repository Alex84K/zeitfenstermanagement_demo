<script setup lang="ts">
import { ref, computed } from 'vue'
import { useBuchungenStore } from '@/stores/buchungen'
import { useStammdatenStore } from '@/stores/stammdaten'
import { ApiError } from '@/api/client'
import type { Rampe, Temperaturbereich } from '@/api/types'

const props = defineProps<{
  rampe: Rampe
  beginnIso: string
}>()

const emit = defineEmits<{ close: []; gebucht: [] }>()

const buchungen = useBuchungenStore()
const stammdaten = useStammdatenStore()

const tz = computed(() => stammdaten.activeStandort?.zeitzone ?? 'Europe/Berlin')

const spediteur = ref('')
const kennzeichen = ref('')
const sendungNr = ref('')
const temperaturbereich = ref<Temperaturbereich>(props.rampe.temperaturbereiche[0] ?? 'TK')
const dauerMin = ref(60)
const fehler = ref<string | null>(null)
const laedt = ref(false)

function formatZeit(iso: string): string {
  return new Date(iso).toLocaleTimeString('de-DE', {
    hour: '2-digit', minute: '2-digit', timeZone: tz.value,
  })
}

const endeISO = computed(() => {
  return new Date(new Date(props.beginnIso).getTime() + dauerMin.value * 60_000).toISOString()
})

async function buchen() {
if (!spediteur.value.trim()) { fehler.value = 'Spediteur ist Pflicht'; return }
  if (!kennzeichen.value.trim()) { fehler.value = 'Kennzeichen ist Pflicht'; return }
  fehler.value = null
  laedt.value = true
  try {
    await buchungen.erstelleBuchung({
      rampe_id: props.rampe.id,
      spediteur: spediteur.value.trim(),
      kennzeichen: kennzeichen.value.trim(),
      sendung_nr: sendungNr.value.trim(),
      temperaturbereich: temperaturbereich.value,
      beginn: props.beginnIso,
      ende: endeISO.value,
    })
    emit('gebucht')
  } catch (e) {
    if (e instanceof ApiError) {
      const body = e.body as { message?: string } | null
      fehler.value = body?.message ?? e.message
    } else {
      fehler.value = 'Netzwerkfehler'
    }
  } finally {
    laedt.value = false
  }
}
</script>

<template>
  <Teleport to="body">
    <div class="modal d-block" tabindex="-1" @click.self="emit('close')">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">
              Buchung erstellen — {{ rampe.bezeichnung }}
            </h5>
            <button type="button" class="btn-close" @click="emit('close')" />
          </div>

          <div class="modal-body">
            <div class="mb-2 text-muted small">
              {{ formatZeit(beginnIso) }} – {{ formatZeit(endeISO) }}
            </div>

            <div v-if="fehler" class="alert alert-danger p-2 small">{{ fehler }}</div>

            <div class="mb-3">
              <label class="form-label">Temperaturbereich</label>
              <select v-model="temperaturbereich" class="form-select form-select-sm">
                <option
                  v-for="t in rampe.temperaturbereiche"
                  :key="t"
                  :value="t"
                >{{ t }}</option>
              </select>
            </div>

            <div class="mb-3">
              <label class="form-label">Dauer</label>
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

            <div class="mb-3">
              <label class="form-label">Kennzeichen <span class="text-danger">*</span></label>
              <input v-model="kennzeichen" type="text" class="form-control" placeholder="GT-ML 1234" @input="fehler = null" />
            </div>

            <div class="mb-3">
              <label class="form-label">Spediteur <span class="text-danger">*</span></label>
              <input v-model="spediteur" type="text" class="form-control" placeholder="Spedition Müller GmbH" @input="fehler = null" />
            </div>

            <div class="mb-3">
              <label class="form-label">Sendungsnummer</label>
              <input v-model="sendungNr" type="text" class="form-control" placeholder="S-2026-…" />
            </div>
          </div>

          <div class="modal-footer">
            <button class="btn btn-secondary" @click="emit('close')">Abbrechen</button>
            <button class="btn btn-primary" :disabled="laedt" @click="buchen">
              {{ laedt ? 'Wird gebucht…' : 'Buchen' }}
            </button>
          </div>
        </div>
      </div>
    </div>
    <div class="modal-backdrop fade show" />
  </Teleport>
</template>

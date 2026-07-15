<script setup lang="ts">
import { ref, watch } from 'vue'
import { useStammdatenStore } from '@/stores/stammdaten'
import { ApiError } from '@/api/client'
import type { Standort, Rampe, Temperaturbereich } from '@/api/types'

defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()

const stammdaten = useStammdatenStore()

type Tab = 'standorte' | 'rampen'
const tab = ref<Tab>('standorte')

const fehler = ref<string | null>(null)

function handleError(e: unknown) {
  if (e instanceof ApiError) {
    const body = e.body as { message?: string } | null
    fehler.value = body?.message ?? e.message
  } else {
    fehler.value = 'Netzwerkfehler'
  }
}

// ──────────────────────────────────────────────
// Standort CRUD
// ──────────────────────────────────────────────
const neuerStandortName = ref('')
const neuerStandortAdresse = ref('')
const neuerStandortZeitzone = ref('Europe/Berlin')
const neuerStandortVon = ref('05:00')
const neuerStandortBis = ref('22:00')
const editStandort = ref<Standort | null>(null)

function startEditStandort(s: Standort) {
  editStandort.value = { ...s }
}

async function speichereNeuerStandort() {
  fehler.value = null
  try {
    await stammdaten.erstelleStandort({
      name: neuerStandortName.value.trim(),
      adresse: neuerStandortAdresse.value.trim(),
      zeitzone: neuerStandortZeitzone.value.trim(),
      oeffnung_von: neuerStandortVon.value,
      oeffnung_bis: neuerStandortBis.value,
    })
    neuerStandortName.value = ''
    neuerStandortAdresse.value = ''
  } catch (e) { handleError(e) }
}

async function speichereEditStandort() {
  if (!editStandort.value) return
  fehler.value = null
  try {
    await stammdaten.aktualisiereStandort(editStandort.value.id, {
      name: editStandort.value.name,
      zeitzone: editStandort.value.zeitzone,
      oeffnung_von: editStandort.value.oeffnung_von,
      oeffnung_bis: editStandort.value.oeffnung_bis,
    })
    editStandort.value = null
  } catch (e) { handleError(e) }
}

// ──────────────────────────────────────────────
// Rampe CRUD
// ──────────────────────────────────────────────
const ALL_TB: Temperaturbereich[] = ['TK', 'FRISCH', 'TROCKEN']

const neueRampeBezeichnung = ref('')
const neueRampeTbs = ref<Temperaturbereich[]>(['TK'])
const editRampe = ref<Rampe | null>(null)
const editRampeTbs = ref<Temperaturbereich[]>([])

function startEditRampe(r: Rampe) {
  editRampe.value = { ...r }
  editRampeTbs.value = [...r.temperaturbereiche]
}

function toggleTb(list: Temperaturbereich[], tb: Temperaturbereich) {
  const idx = list.indexOf(tb)
  if (idx === -1) list.push(tb)
  else list.splice(idx, 1)
}

async function speichereNeueRampe() {
  if (!stammdaten.activeStandortId) return
  fehler.value = null
  try {
    await stammdaten.erstelleRampe(stammdaten.activeStandortId, {
      bezeichnung: neueRampeBezeichnung.value.trim(),
      temperaturbereiche: [...neueRampeTbs.value],
    })
    neueRampeBezeichnung.value = ''
    neueRampeTbs.value = ['TK']
  } catch (e) { handleError(e) }
}

async function speichereEditRampe() {
  if (!editRampe.value) return
  fehler.value = null
  try {
    await stammdaten.aktualisiereRampe(editRampe.value.id, {
      bezeichnung: editRampe.value.bezeichnung,
      aktiv: editRampe.value.aktiv,
      temperaturbereiche: [...editRampeTbs.value],
    })
    editRampe.value = null
  } catch (e) { handleError(e) }
}

// Reset errors when switching tabs
watch(tab, () => { fehler.value = null; editStandort.value = null; editRampe.value = null })
</script>

<template>
  <Teleport to="body">
    <!-- Backdrop -->
    <div
      v-if="open"
      class="offcanvas-backdrop fade show"
      @click="emit('close')"
    />

    <!-- Offcanvas panel -->
    <div
      class="offcanvas offcanvas-end"
      :class="{ show: open }"
      tabindex="-1"
      style="width:420px;visibility:visible"
    >
      <div class="offcanvas-header border-bottom">
        <h5 class="offcanvas-title">Stammdaten</h5>
        <button type="button" class="btn-close" @click="emit('close')" />
      </div>

      <!-- Tabs -->
      <ul class="nav nav-tabs px-3 pt-2">
        <li class="nav-item">
          <button
            class="nav-link"
            :class="{ active: tab === 'standorte' }"
            @click="tab = 'standorte'"
          >Standorte</button>
        </li>
        <li class="nav-item">
          <button
            class="nav-link"
            :class="{ active: tab === 'rampen' }"
            @click="tab = 'rampen'"
          >Rampen</button>
        </li>
      </ul>

      <div class="offcanvas-body overflow-y-auto">
        <div v-if="fehler" class="alert alert-danger p-2 small mb-3">{{ fehler }}</div>

        <!-- ── Standorte ── -->
        <div v-if="tab === 'standorte'">
          <!-- Liste -->
          <div class="mb-3">
            <div
              v-for="s in stammdaten.standorte"
              :key="s.id"
              class="border rounded p-2 mb-2"
            >
              <template v-if="editStandort?.id === s.id">
                <div class="d-flex flex-column gap-2">
                  <input v-model="editStandort!.name" class="form-control form-control-sm" placeholder="Name" />
                  <input v-model="editStandort!.zeitzone" class="form-control form-control-sm" placeholder="Europe/Berlin" />
                  <div class="d-flex gap-2">
                    <div class="flex-grow-1">
                      <label class="form-label mb-0 small">Von</label>
                      <input v-model="editStandort!.oeffnung_von" type="time" class="form-control form-control-sm" />
                    </div>
                    <div class="flex-grow-1">
                      <label class="form-label mb-0 small">Bis</label>
                      <input v-model="editStandort!.oeffnung_bis" type="time" class="form-control form-control-sm" />
                    </div>
                  </div>
                  <div class="d-flex gap-2">
                    <button class="btn btn-sm btn-primary" @click="speichereEditStandort">Speichern</button>
                    <button class="btn btn-sm btn-secondary" @click="editStandort = null">Abbrechen</button>
                  </div>
                </div>
              </template>
              <template v-else>
                <div class="d-flex justify-content-between align-items-start">
                  <div>
                    <div class="fw-semibold">{{ s.name }}</div>
                    <div class="text-muted small">{{ s.zeitzone }} · {{ s.oeffnung_von }}–{{ s.oeffnung_bis }}</div>
                  </div>
                  <button class="btn btn-sm btn-outline-secondary" @click="startEditStandort(s)">Bearbeiten</button>
                </div>
              </template>
            </div>
          </div>

          <!-- Neu -->
          <div class="border rounded p-2">
            <div class="fw-semibold small mb-2">Neuer Standort</div>
            <div class="d-flex flex-column gap-2">
              <input v-model="neuerStandortName" class="form-control form-control-sm" placeholder="Name" />
              <input v-model="neuerStandortAdresse" class="form-control form-control-sm" placeholder="Adresse" />
              <input v-model="neuerStandortZeitzone" class="form-control form-control-sm" placeholder="Europe/Berlin" />
              <div class="d-flex gap-2">
                <div class="flex-grow-1">
                  <label class="form-label mb-0 small">Öffnung von</label>
                  <input v-model="neuerStandortVon" type="time" class="form-control form-control-sm" />
                </div>
                <div class="flex-grow-1">
                  <label class="form-label mb-0 small">Öffnung bis</label>
                  <input v-model="neuerStandortBis" type="time" class="form-control form-control-sm" />
                </div>
              </div>
              <button class="btn btn-sm btn-success" :disabled="!neuerStandortName.trim()" @click="speichereNeuerStandort">
                Standort anlegen
              </button>
            </div>
          </div>
        </div>

        <!-- ── Rampen ── -->
        <div v-if="tab === 'rampen'">
          <div v-if="!stammdaten.activeStandortId" class="text-muted small">
            Bitte zuerst einen Standort auswählen.
          </div>
          <template v-else>
            <!-- Liste -->
            <div class="mb-3">
              <div
                v-for="r in stammdaten.rampen"
                :key="r.id"
                class="border rounded p-2 mb-2"
                :class="{ 'opacity-50': !r.aktiv }"
              >
                <template v-if="editRampe?.id === r.id">
                  <div class="d-flex flex-column gap-2">
                    <input v-model="editRampe!.bezeichnung" class="form-control form-control-sm" placeholder="Bezeichnung" />
                    <div>
                      <label class="form-label mb-1 small">Temperaturbereiche</label>
                      <div class="d-flex gap-2">
                        <div v-for="tb in ALL_TB" :key="tb" class="form-check form-check-inline">
                          <input
                            :id="`edit-tb-${tb}`"
                            class="form-check-input"
                            type="checkbox"
                            :checked="editRampeTbs.includes(tb)"
                            @change="toggleTb(editRampeTbs, tb)"
                          />
                          <label :for="`edit-tb-${tb}`" class="form-check-label small">{{ tb }}</label>
                        </div>
                      </div>
                    </div>
                    <div class="form-check">
                      <input
                        id="edit-aktiv"
                        v-model="editRampe!.aktiv"
                        class="form-check-input"
                        type="checkbox"
                      />
                      <label for="edit-aktiv" class="form-check-label small">Aktiv</label>
                    </div>
                    <div class="d-flex gap-2">
                      <button class="btn btn-sm btn-primary" @click="speichereEditRampe">Speichern</button>
                      <button class="btn btn-sm btn-secondary" @click="editRampe = null">Abbrechen</button>
                    </div>
                  </div>
                </template>
                <template v-else>
                  <div class="d-flex justify-content-between align-items-start">
                    <div>
                      <div class="fw-semibold">{{ r.bezeichnung }}</div>
                      <div class="text-muted small">
                        {{ r.temperaturbereiche.join(', ') }}
                        <span v-if="!r.aktiv" class="text-danger ms-1">· inaktiv</span>
                      </div>
                    </div>
                    <button class="btn btn-sm btn-outline-secondary" @click="startEditRampe(r)">Bearbeiten</button>
                  </div>
                </template>
              </div>
              <div v-if="stammdaten.rampen.length === 0" class="text-muted small">
                Keine Rampen vorhanden.
              </div>
            </div>

            <!-- Neu -->
            <div class="border rounded p-2">
              <div class="fw-semibold small mb-2">Neue Rampe</div>
              <div class="d-flex flex-column gap-2">
                <input v-model="neueRampeBezeichnung" class="form-control form-control-sm" placeholder="Tor 01" />
                <div>
                  <label class="form-label mb-1 small">Temperaturbereiche</label>
                  <div class="d-flex gap-2">
                    <div v-for="tb in ALL_TB" :key="tb" class="form-check form-check-inline">
                      <input
                        :id="`new-tb-${tb}`"
                        class="form-check-input"
                        type="checkbox"
                        :checked="neueRampeTbs.includes(tb)"
                        @change="toggleTb(neueRampeTbs, tb)"
                      />
                      <label :for="`new-tb-${tb}`" class="form-check-label small">{{ tb }}</label>
                    </div>
                  </div>
                </div>
                <button
                  class="btn btn-sm btn-success"
                  :disabled="!neueRampeBezeichnung.trim() || neueRampeTbs.length === 0"
                  @click="speichereNeueRampe"
                >
                  Rampe anlegen
                </button>
              </div>
            </div>
          </template>
        </div>
      </div>
    </div>
  </Teleport>
</template>

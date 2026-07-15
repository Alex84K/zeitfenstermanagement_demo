import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import { api } from '@/api/client'
import type { Standort, Rampe } from '@/api/types'

export const useStammdatenStore = defineStore('stammdaten', () => {
  const standorte = ref<Standort[]>([])
  const activeStandortId = ref<number | null>(null)
  const rampen = ref<Rampe[]>([])

  const activeStandort = computed(
    () => standorte.value.find((s) => s.id === activeStandortId.value) ?? null,
  )

  async function ladeStandorte() {
    standorte.value = await api.get<Standort[]>('/standorte')
    if (standorte.value.length > 0 && activeStandortId.value === null) {
      activeStandortId.value = standorte.value[0].id
    }
    if (activeStandortId.value !== null) {
      await ladeRampen(activeStandortId.value)
    }
  }

  async function ladeRampen(id: number) {
    rampen.value = await api.get<Rampe[]>(`/standorte/${id}/rampen`)
  }

  async function waehleStandort(id: number) {
    activeStandortId.value = id
    await ladeRampen(id)
  }

  async function erstelleStandort(data: {
    name: string
    adresse: string
    zeitzone: string
    oeffnung_von: string
    oeffnung_bis: string
  }) {
    const s = await api.post<Standort>('/standorte', data)
    standorte.value.push(s)
    return s
  }

  async function aktualisiereStandort(
    id: number,
    data: Partial<{
      name: string
      adresse: string
      zeitzone: string
      oeffnung_von: string
      oeffnung_bis: string
      aktiv: boolean
    }>,
  ) {
    const s = await api.patch<Standort>(`/standorte/${id}`, data)
    const idx = standorte.value.findIndex((x) => x.id === id)
    if (idx !== -1) standorte.value[idx] = s
    return s
  }

  async function erstelleRampe(
    standortId: number,
    data: { bezeichnung: string; temperaturbereiche: string[] },
  ) {
    const r = await api.post<Rampe>(`/standorte/${standortId}/rampen`, data)
    if (standortId === activeStandortId.value) rampen.value.push(r)
    return r
  }

  async function aktualisiereRampe(
    id: number,
    data: Partial<{ bezeichnung: string; temperaturbereiche: string[]; aktiv: boolean }>,
  ) {
    const r = await api.patch<Rampe>(`/rampen/${id}`, data)
    const idx = rampen.value.findIndex((x) => x.id === id)
    if (idx !== -1) rampen.value[idx] = r
    return r
  }

  return {
    standorte,
    activeStandortId,
    activeStandort,
    rampen,
    ladeStandorte,
    ladeRampen,
    waehleStandort,
    erstelleStandort,
    aktualisiereStandort,
    erstelleRampe,
    aktualisiereRampe,
  }
})

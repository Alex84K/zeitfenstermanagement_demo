import { ref } from 'vue'
import { defineStore } from 'pinia'
import { api } from '@/api/client'
import { useStammdatenStore } from './stammdaten'
import type { Buchung, Kennzahlen, EreignisTyp, CreateBuchungReq, VerfuegbarkeitsSlot, Temperaturbereich } from '@/api/types'

function todayISO(): string {
  return new Date().toLocaleDateString('sv') // sv locale gives YYYY-MM-DD
}

export const useBuchungenStore = defineStore('buchungen', () => {
  const stammdaten = useStammdatenStore()

  const datum = ref(todayISO())
  const buchungen = ref<Buchung[]>([])
  const kennzahlen = ref<Kennzahlen | null>(null)
  const laedt = ref(false)

  async function laden() {
    const id = stammdaten.activeStandortId
    if (id === null) return
    laedt.value = true
    try {
      const [bs, kz] = await Promise.all([
        api.get<Buchung[]>('/buchungen', { query: { standort_id: id, datum: datum.value } }),
        api.get<Kennzahlen>('/kennzahlen', { query: { standort_id: id, datum: datum.value } }),
      ])
      buchungen.value = bs ?? []
      kennzahlen.value = kz
    } finally {
      laedt.value = false
    }
  }

  async function sucheMeineBuchungen(spediteur: string): Promise<Buchung[]> {
    return api.get<Buchung[]>('/buchungen', {
      query: { spediteur, ab: new Date().toISOString() },
    })
  }

  async function sucheVerfuegbarkeit(
    temperaturbereich: Temperaturbereich,
    dauerMin: number,
  ): Promise<VerfuegbarkeitsSlot[]> {
    const id = stammdaten.activeStandortId
    if (id === null) return []
    return api.get<VerfuegbarkeitsSlot[]>('/verfuegbarkeit', {
      query: {
        standort_id: id,
        datum: datum.value,
        temperaturbereich,
        dauer: dauerMin,
      },
    })
  }

  async function erstelleBuchung(req: CreateBuchungReq) {
    await api.post<Buchung>('/buchungen', req)
    await laden()
  }

  async function stornieren(id: number) {
    await api.delete<void>(`/buchungen/${id}`)
    await laden()
  }

  async function erfasseEreignis(buchungId: number, typ: EreignisTyp) {
    await api.post<void>(`/buchungen/${buchungId}/ereignisse`, {
      typ,
      erfasst_von: 'Pförtner',
    })
    await laden()
  }

  function setDatum(d: string) {
    datum.value = d
  }

  return {
    datum,
    buchungen,
    kennzahlen,
    laedt,
    laden,
    setDatum,
    sucheMeineBuchungen,
    sucheVerfuegbarkeit,
    erstelleBuchung,
    stornieren,
    erfasseEreignis,
  }
})

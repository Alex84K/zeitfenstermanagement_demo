export type Temperaturbereich = 'TK' | 'FRISCH' | 'TROCKEN'
export type EreignisTyp = 'ANGEKOMMEN' | 'ANGEDOCKT' | 'ABGEFAHREN'
export type BuchungStatus =
  | 'GEBUCHT'
  | 'ANGEKOMMEN'
  | 'ANGEDOCKT'
  | 'ABGEFAHREN'
  | 'NO_SHOW'
  | 'STORNIERT'

export interface Standort {
  id: number
  name: string
  adresse: string
  zeitzone: string
  oeffnung_von: string
  oeffnung_bis: string
  aktiv: boolean
}

export interface Rampe {
  id: number
  standort_id: number
  bezeichnung: string
  temperaturbereiche: Temperaturbereich[]
  aktiv: boolean
}

export interface Buchung {
  id: number
  rampe_id: number
  spediteur: string
  kennzeichen: string
  sendung_nr: string
  temperaturbereich: Temperaturbereich
  beginn: string
  ende: string
  status: BuchungStatus
}

export interface Ereignis {
  typ: EreignisTyp
  zeitpunkt: string
}

export interface BuchungDetail extends Buchung {
  ereignisse: Ereignis[]
}

export interface VerfuegbarkeitsSlot {
  rampe_id: number
  bezeichnung: string
  beginn: string
  ende: string
}

export interface Kennzahlen {
  buchungen_gesamt: number
  standzeit_schnitt_min: number | null
  puenktlichkeit_prozent: number | null
  no_shows: number
}

export interface CreateBuchungReq {
  rampe_id: number
  spediteur: string
  kennzeichen: string
  sendung_nr: string
  temperaturbereich: Temperaturbereich
  beginn: string
  ende: string
}

export interface ApiErrorBody {
  code: string
  message: string
}

package zeitfenster

import "errors"

// Sentinel-Fehler des Domains. Siehe MVP.md §6 fuer die vollstaendige Regel-Tabelle
// und internal/httpapi/fehler.go fuer das Mapping auf HTTP-Status + maschinenlesbaren `code`.
var (
	// Buchung erstellen (Regeln 2-7).
	ErrTemperaturMismatch      = errors.New("shipment temperature zone incompatible with rampe")
	ErrAusserhalbOeffnungszeit = errors.New("buchung outside standort opening hours")
	ErrNichtImRaster           = errors.New("buchung not aligned to 30-minute slot grid")
	ErrUngueltigeDauer         = errors.New("buchung duration must be between 30 minutes and 4 hours")
	ErrVergangenheit           = errors.New("cannot book in the past")
	ErrRampeInaktiv            = errors.New("rampe or standort is inactive")

	// Konflikt beim Anlegen (Regel 1) — wird von internal/sqlite anhand des
	// Constraint-Verstosses erkannt und als dieser Fehler zurueckgegeben.
	ErrRampeBelegt = errors.New("rampe already booked for this slot")

	// Buchung finden/stornieren.
	ErrBuchungNotFound = errors.New("buchung not found")

	// Stammdaten finden.
	ErrStandortNotFound = errors.New("standort not found")
	ErrRampeNotFound    = errors.New("rampe not found")

	// Ereignisse (Regeln 8-12).
	ErrUngueltigerStatuswechsel = errors.New("invalid gate event transition")
	ErrBuchungStorniert         = errors.New("buchung is cancelled, no gate events allowed")

	// Stammdaten (Regeln 14-20).
	ErrRampeHatBuchungen     = errors.New("rampe has future active buchungen")
	ErrStandortHatRampen     = errors.New("standort has active rampen")
	ErrUngueltigeZeitzone    = errors.New("invalid IANA timezone")
	ErrKeinTemperaturbereich = errors.New("rampe must support at least one temperaturbereich")
	ErrBezeichnungBelegt     = errors.New("bezeichnung already used within this standort")
)

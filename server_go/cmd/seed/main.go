// cmd/seed наполняет сегодняшний день демо-данными, чтобы консоль с первой
// секунды выглядела работающей: часть броней уже ABGEFAHREN (KPI показывает
// настоящие числа), одна ANGEKOMMEN, одна ANGEDOCKT, один NO_SHOW и несколько
// будущих GEBUCHT. Без сидера демо стартует с пустой сеткой — MVP.md §8.
//
// Идемпотентность: если в БД уже есть хоть один Standort, сидер выходит без
// изменений. Запускать повторно без сброса БД бессмысленно.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"zeitfenster/internal/sqlite"
	"zeitfenster/internal/zeitfenster"
)

const berlinTZ = "Europe/Berlin"

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	db, err := sqlite.Open(getenv("ZFM_DB_PATH", "./zeitfenster.db"))
	if err != nil {
		log.Error("open db", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx := context.Background()
	standortStore := sqlite.NewStandortStore(db)
	rampenStore := sqlite.NewRampeStore(db)
	buchungStore := sqlite.NewBuchungStore(db)
	ereignisStore := sqlite.NewEreignisStore(db)

	vorhandene, err := standortStore.List(ctx)
	mustDo(log, err, "list standorte")
	if len(vorhandene) > 0 {
		log.Info("bereits befüllt – übersprungen", "standorte", len(vorhandene))
		return
	}

	// --- Stammdaten ---

	standort, err := zeitfenster.NewStandort(
		0, "Versmold", "Gütersloher Str. 111, 33775 Versmold",
		berlinTZ, 5*time.Hour, 22*time.Hour, true,
	)
	mustDo(log, err, "new standort")
	mustDo(log, standortStore.Create(ctx, &standort), "create standort")
	log.Info("Standort angelegt", "id", standort.ID, "name", standort.Name)

	loc := standort.Zeitzone

	type rSpec struct {
		name    string
		bereiche []zeitfenster.Temperaturbereich
	}
	rampeSpecs := []rSpec{
		{"Tor 01", []zeitfenster.Temperaturbereich{zeitfenster.TK}},
		{"Tor 02", []zeitfenster.Temperaturbereich{zeitfenster.TK}},
		{"Tor 03", []zeitfenster.Temperaturbereich{zeitfenster.TK}},
		{"Tor 04", []zeitfenster.Temperaturbereich{zeitfenster.Frisch}},
		{"Tor 05", []zeitfenster.Temperaturbereich{zeitfenster.Frisch}},
		{"Tor 06", []zeitfenster.Temperaturbereich{zeitfenster.Trocken}},
		{"Tor 07", []zeitfenster.Temperaturbereich{zeitfenster.Trocken}},
		{"Tor 08", []zeitfenster.Temperaturbereich{zeitfenster.TK, zeitfenster.Frisch}},
	}
	rampen := make([]zeitfenster.Rampe, 0, len(rampeSpecs))
	for _, s := range rampeSpecs {
		r, err := zeitfenster.NewRampe(0, standort.ID, s.name,
			zeitfenster.NewTemperaturbereichSet(s.bereiche...), true)
		mustDo(log, err, "new rampe")
		mustDo(log, rampenStore.Create(ctx, &r), "create rampe")
		rampen = append(rampen, r)
		log.Info("Rampe angelegt", "id", r.ID, "bezeichnung", r.Bezeichnung)
	}

	tor01, tor02, tor03 := rampen[0], rampen[1], rampen[2]
	tor04, tor05 := rampen[3], rampen[4]
	tor06, tor07, tor08 := rampen[5], rampen[6], rampen[7]

	// --- Zeitberechnungen ---

	heute := time.Now().In(loc)

	// bt gibt eine Uhrzeit am heutigen Berliner Kalendertag als UTC zurück.
	bt := func(h, m int) time.Time {
		return time.Date(heute.Year(), heute.Month(), heute.Day(), h, m, 0, 0, loc).UTC()
	}

	jetzt := time.Now().UTC()

	// Zukunftsbuchungen beginnen frühestens um 11:00 (nach allen Vergangenheitsbuchungen)
	// und frühestens ab der nächsten 30-Minuten-Grenze nach jetzt.
	minFuture := bt(11, 0)
	nextBoundary := jetzt.In(loc).Truncate(30 * time.Minute).Add(30 * time.Minute).UTC()
	if nextBoundary.After(minFuture) {
		minFuture = nextBoundary
	}
	schluss := bt(22, 0) // Öffnungsende Versmold

	// futureSlot gibt Beginn und Ende des n-ten Zukunftsslots zurück (Schritte à 30 min).
	// ok=false, wenn das Ende die Öffnungszeiten überschreiten würde.
	futureSlot := func(n int, dur time.Duration) (beginn, ende time.Time, ok bool) {
		b := minFuture.Add(time.Duration(n) * 30 * time.Minute)
		e := b.Add(dur)
		return b, e, !e.After(schluss)
	}

	// --- Buchungstypen ---

	type ereignisSeed struct {
		typ    zeitfenster.EreignisTyp
		offset time.Duration // relativ zu Buchung.Beginn
	}
	type buchungSeed struct {
		rampe      zeitfenster.Rampe
		spediteur  string
		kz         string
		sendung    string
		temp       zeitfenster.Temperaturbereich
		beginn     time.Time
		ende       time.Time
		ereignisse []ereignisSeed
	}

	// --- Vergangenheitsbuchungen (feste Berliner Uhrzeiten, immer in der Vergangenheit) ---

	vergangenheit := []buchungSeed{
		// ABGEFAHREN – pünktlich, Standzeit 58 min
		{
			rampe: tor01, spediteur: "Spedition Müller GmbH", kz: "GT-ML 1234",
			sendung: "S-2026-00781", temp: zeitfenster.TK,
			beginn: bt(6, 0), ende: bt(7, 0),
			ereignisse: []ereignisSeed{
				{zeitfenster.Angekommen, -7 * time.Minute},
				{zeitfenster.Angedockt, +9 * time.Minute},
				{zeitfenster.Abgefahren, +51 * time.Minute},
			},
		},
		// ABGEFAHREN – pünktlich, Standzeit 63 min
		{
			rampe: tor04, spediteur: "Kühllogistik Weber", kz: "BI-KW 4567",
			sendung: "S-2026-01023", temp: zeitfenster.Frisch,
			beginn: bt(6, 30), ende: bt(7, 30),
			ereignisse: []ereignisSeed{
				{zeitfenster.Angekommen, -8 * time.Minute},
				{zeitfenster.Angedockt, +7 * time.Minute},
				{zeitfenster.Abgefahren, +55 * time.Minute},
			},
		},
		// ABGEFAHREN – UNPÜNKTLICH: +19 min > Karenzzeit 15 min
		{
			rampe: tor06, spediteur: "Bauer & Söhne Spedition", kz: "MS-BA 7890",
			sendung: "S-2026-01156", temp: zeitfenster.Trocken,
			beginn: bt(7, 0), ende: bt(8, 30),
			ereignisse: []ereignisSeed{
				{zeitfenster.Angekommen, +19 * time.Minute},
				{zeitfenster.Angedockt, +31 * time.Minute},
				{zeitfenster.Abgefahren, +82 * time.Minute},
			},
		},
		// ABGEFAHREN – pünktlich, Standzeit 68 min
		{
			rampe: tor02, spediteur: "NordFreeze Logistics GmbH", kz: "HH-NF 5678",
			sendung: "S-2026-01289", temp: zeitfenster.TK,
			beginn: bt(7, 30), ende: bt(9, 0),
			ereignisse: []ereignisSeed{
				{zeitfenster.Angekommen, +12 * time.Minute}, // +12 min ≤ 15 min → pünktlich
				{zeitfenster.Angedockt, +27 * time.Minute},
				{zeitfenster.Abgefahren, +68 * time.Minute},
			},
		},
		// NO_SHOW – kein Ereignis, Slot lief ab; zeigt sich im KPI
		{
			rampe: tor03, spediteur: "Transport Nord GmbH", kz: "OS-TN 2345",
			sendung: "S-2026-01334", temp: zeitfenster.TK,
			beginn: bt(8, 0), ende: bt(9, 0),
			ereignisse: nil,
		},
		// ANGEKOMMEN – auf dem Gelände, wartet auf Rampenzuweisung
		{
			rampe: tor05, spediteur: "FrischeSprint KG", kz: "DO-FS 3456",
			sendung: "S-2026-01445", temp: zeitfenster.Frisch,
			beginn: bt(9, 0), ende: bt(10, 30),
			ereignisse: []ereignisSeed{
				{zeitfenster.Angekommen, -6 * time.Minute},
			},
		},
		// ANGEDOCKT – steht unter der Rampe
		{
			rampe: tor07, spediteur: "Logistik König & Partner", kz: "DU-LK 6789",
			sendung: "S-2026-01512", temp: zeitfenster.Trocken,
			beginn: bt(9, 30), ende: bt(11, 0),
			ereignisse: []ereignisSeed{
				{zeitfenster.Angekommen, -3 * time.Minute},
				{zeitfenster.Angedockt, +12 * time.Minute},
			},
		},
	}

	// --- Zukunftsbuchungen (dynamisch ab minFuture) ---

	type futureSpec struct {
		rampe     zeitfenster.Rampe
		spediteur string
		kz        string
		sendung   string
		temp      zeitfenster.Temperaturbereich
		n         int           // Slot-Index (Schritte à 30 min ab minFuture)
		dur       time.Duration
	}
	futureSpecs := []futureSpec{
		{tor01, "Tiefkühl Nord GmbH", "KI-TN 1111", "S-2026-01678", zeitfenster.TK, 0, 60 * time.Minute},
		{tor04, "Frische Allee GmbH", "MA-FA 2222", "S-2026-01734", zeitfenster.Frisch, 1, 60 * time.Minute},
		{tor02, "PolarSpeed AG", "PB-PS 3333", "S-2026-01801", zeitfenster.TK, 1, 90 * time.Minute},
		{tor06, "Trockentransport West KG", "ST-TW 4444", "S-2026-01867", zeitfenster.Trocken, 2, 60 * time.Minute},
		{tor08, "MultiKühl Logistik GmbH", "RE-MK 5555", "S-2026-01923", zeitfenster.Frisch, 2, 60 * time.Minute},
		{tor03, "ArcticExpress GmbH", "WI-AE 6666", "S-2026-01989", zeitfenster.TK, 3, 60 * time.Minute},
	}

	zukunft := make([]buchungSeed, 0, len(futureSpecs))
	for _, fs := range futureSpecs {
		b, e, ok := futureSlot(fs.n, fs.dur)
		if !ok {
			log.Info("Slot hinter Schließzeit – übersprungen", "kz", fs.kz,
				"wäre_beginn", b.In(loc).Format("15:04"))
			continue
		}
		zukunft = append(zukunft, buchungSeed{
			rampe: fs.rampe, spediteur: fs.spediteur, kz: fs.kz,
			sendung: fs.sendung, temp: fs.temp,
			beginn: b, ende: e,
		})
	}

	// --- Einfügen ---

	alle := append(vergangenheit, zukunft...)
	for _, s := range alle {
		b := zeitfenster.Buchung{
			RampeID:           s.rampe.ID,
			Spediteur:         s.spediteur,
			Kennzeichen:       s.kz,
			SendungNr:         s.sendung,
			Temperaturbereich: s.temp,
			Beginn:            s.beginn,
			Ende:              s.ende,
			ErstelltAm:        s.beginn.Add(-18 * time.Hour), // gestern gebucht
		}
		mustDo(log, buchungStore.Create(ctx, &b), "create buchung "+s.kz)

		for _, ev := range s.ereignisse {
			e := zeitfenster.BuchungEreignis{
				BuchungID:  b.ID,
				Typ:        ev.typ,
				Zeitpunkt:  s.beginn.Add(ev.offset),
				ErfasstVon: "Seed",
			}
			mustDo(log, ereignisStore.Append(ctx, &e), "append ereignis")
		}

		statusHinweis := "GEBUCHT"
		if len(s.ereignisse) == 3 {
			statusHinweis = "ABGEFAHREN"
		} else if len(s.ereignisse) == 2 {
			statusHinweis = "ANGEDOCKT"
		} else if len(s.ereignisse) == 1 {
			statusHinweis = "ANGEKOMMEN"
		} else if b.Beginn.Before(jetzt.Add(-zeitfenster.Karenzzeit)) {
			statusHinweis = "NO_SHOW"
		}
		log.Info("Buchung angelegt",
			"id", b.ID,
			"kz", s.kz,
			"rampe", s.rampe.Bezeichnung,
			"beginn", s.beginn.In(loc).Format("15:04"),
			"ende", s.ende.In(loc).Format("15:04"),
			"status", statusHinweis,
		)
	}

	log.Info("Seed abgeschlossen",
		"standort", standort.Name,
		"rampen", len(rampen),
		"buchungen", len(alle),
	)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustDo(log *slog.Logger, err error, op string) {
	if err != nil {
		log.Error(op, "err", err)
		os.Exit(1)
	}
}

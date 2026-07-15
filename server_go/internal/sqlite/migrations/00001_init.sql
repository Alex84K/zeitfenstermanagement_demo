-- +goose Up

-- Standort: Lager-Niederlassung. oeffnung_*_min sind Minuten ab lokaler
-- Mitternacht (05:00 = 300) — passt direkt auf zeitfenster.Standort.OeffnungVon
-- als time.Duration (Minuten * time.Minute). aktiv statt DELETE: Regel 14.
CREATE TABLE standort (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    name             TEXT    NOT NULL,
    adresse          TEXT    NOT NULL DEFAULT '',
    zeitzone         TEXT    NOT NULL,
    oeffnung_von_min INTEGER NOT NULL,
    oeffnung_bis_min INTEGER NOT NULL,
    aktiv            INTEGER NOT NULL DEFAULT 1 CHECK (aktiv IN (0, 1))
);

-- Rampe: Verladetor an einem Standort. UNIQUE(standort_id, bezeichnung) ist Regel 18.
CREATE TABLE rampe (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    standort_id INTEGER NOT NULL REFERENCES standort (id),
    bezeichnung TEXT    NOT NULL,
    aktiv       INTEGER NOT NULL DEFAULT 1 CHECK (aktiv IN (0, 1)),
    UNIQUE (standort_id, bezeichnung)
);

CREATE INDEX idx_rampe_standort ON rampe (standort_id);

-- rampe_temperaturbereich: die Menge der von einer Rampe unterstuetzten Zonen
-- (zeitfenster.TemperaturbereichSet). Mindestens ein Eintrag ist Regel 20 —
-- das erzwingt NewRampe() im Domain, nicht die Datenbank, da es beim Anlegen
-- einer Rampe mit ihren Zonen atomar in derselben Transaktion passiert.
CREATE TABLE rampe_temperaturbereich (
    rampe_id          INTEGER NOT NULL REFERENCES rampe (id) ON DELETE CASCADE,
    temperaturbereich TEXT    NOT NULL CHECK (temperaturbereich IN ('TK', 'FRISCH', 'TROCKEN')),
    PRIMARY KEY (rampe_id, temperaturbereich)
) WITHOUT ROWID;

-- Buchung: die Reservierung selbst. beginn/ende/erstellt_am/storniert_am sind
-- ISO-8601 UTC als TEXT (CONVENTIONS.md §8) — lexikographische Sortierung
-- entspricht der chronologischen, BETWEEN und < funktionieren direkt auf TEXT.
-- Kein status-Feld: der Status wird aus buchung_ereignis berechnet (MVP.md §5).
CREATE TABLE buchung (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    rampe_id          INTEGER NOT NULL REFERENCES rampe (id),
    spediteur         TEXT    NOT NULL,
    kennzeichen       TEXT    NOT NULL,
    sendung_nr        TEXT    NOT NULL DEFAULT '',
    temperaturbereich TEXT    NOT NULL CHECK (temperaturbereich IN ('TK', 'FRISCH', 'TROCKEN')),
    beginn            TEXT    NOT NULL,
    ende              TEXT    NOT NULL,
    erstellt_am       TEXT    NOT NULL,
    storniert_am      TEXT
);

CREATE INDEX idx_buchung_rampe_beginn ON buchung (rampe_id, beginn);
CREATE INDEX idx_buchung_spediteur ON buchung (spediteur);

-- buchung_slot: eine Zeile pro belegtem 30-Minuten-Slot (zeitfenster.Slots()).
-- Der PRIMARY KEY hier IST Regel 1 — zwei Buchungen koennen denselben Slot
-- derselben Rampe nicht gleichzeitig belegen, race-frei durch die Datenbank
-- erzwungen. Siehe MVP.md §6 und CONVENTIONS.md §8: eine SELECT-Pruefung vorher
-- waere eine Race Condition, dieser Constraint ist es nicht.
CREATE TABLE buchung_slot (
    rampe_id    INTEGER NOT NULL REFERENCES rampe (id),
    slot_beginn TEXT    NOT NULL,
    buchung_id  INTEGER NOT NULL REFERENCES buchung (id) ON DELETE CASCADE,
    PRIMARY KEY (rampe_id, slot_beginn)
) WITHOUT ROWID;

-- buchung_ereignis: Append-only-Journal physischer Tor-Ereignisse.
-- UNIQUE(buchung_id, typ) ist Regel 12 — jedes Ereignis hoechstens einmal.
-- NO_SHOW erscheint hier absichtlich nicht: es ist kein beobachtetes Ereignis,
-- sondern eine Berechnung aus dem Fehlen eines Ereignisses (MVP.md §5, §6).
CREATE TABLE buchung_ereignis (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    buchung_id  INTEGER NOT NULL REFERENCES buchung (id) ON DELETE CASCADE,
    typ         TEXT    NOT NULL CHECK (typ IN ('ANGEKOMMEN', 'ANGEDOCKT', 'ABGEFAHREN')),
    zeitpunkt   TEXT    NOT NULL,
    erfasst_von TEXT    NOT NULL DEFAULT '',
    UNIQUE (buchung_id, typ)
);

-- +goose Down

DROP TABLE buchung_ereignis;
DROP TABLE buchung_slot;
DROP TABLE buchung;
DROP TABLE rampe_temperaturbereich;
DROP TABLE rampe;
DROP TABLE standort;

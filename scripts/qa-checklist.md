# QA-sjekkliste: Respond+ (Respond.exe)

**Bruk denne:** etter hver `wails build`, før du anser en endring "ferdig testet". Ta 5-10 minutter, ikke skip den fordi endringen "var liten" — lyd-regresjon har skjedd flere ganger fra små endringer.

**Testere:** minimum 2 personer/klienter (én i Wails, én i nettleser) — mange feil (asymmetrisk lyd) vises kun med to samtidige klienter.

---

## 1. Bygg og oppstart (teknisk)

| # | Test | Forventet | Pass/Fail | Notat |
|---|------|-----------|-----------|-------|
| 1.1 | `git pull` kjørt før build | "Already up to date" eller viser nye filer | ☐ |  |
| 1.2 | `wails build` fullfører uten feil | "Built 'D:\...\Respond.exe'" | ☐ |  |
| 1.3 | Tittel viser riktig versjon | `Respond vX.XX [YYYYMMDD-tag]` matcher siste commit | ☐ |  |
| 1.4 | Port 8080 fri før start | `netstat -ano | findstr ":8080"` → tomt | ☐ |  |
| 1.5 | `.\build\bin\Respond.exe` åpner vindu | Login-skjerm vises innen 3 sek | ☐ |  |
| 1.6 | Node health svarer | `Invoke-WebRequest http://127.0.0.1:8080/api/v1/node/health` returnerer JSON | ☐ |  |

## 2. Innlogging og tilkobling

| # | Test | Forventet | Pass/Fail | Notat |
|---|------|-----------|-----------|-------|
| 2.1 | Login som bruker A (Wails) | WS kobler til, presence vises | ☐ |  |
| 2.2 | Login som bruker B (nettleser, `localhost:8080`, hard refresh) | Samme — ingen 404 | ☐ |  |
| 2.3 | Begge ser hverandre i brukerlisten | Riktig kanal (default `lobby`) | ☐ |  |
| 2.4 | Terminal viser `ws: connected user=...` for begge | To linjer, riktig userId/navn | ☐ |  |

## 3. Lyd — kritisk sti, test BÅDE retninger separat

| # | Test | Forventet | Pass/Fail | Notat |
|---|------|-----------|-----------|-------|
| 3.1 | Bruker A (Wails) snakker → bruker B (nettleser) hører | Klar lyd, ingen kutt | ☐ |  |
| 3.2 | Bruker B (nettleser) snakker → bruker A (Wails) hører | Klar lyd, ingen kutt | ☐ | **Test denne separat — har historisk feilet uavhengig av 3.1** |
| 3.3 | Terminal viser `sfu: uplink track` ved tale | Vises for begge brukere når de snakker | ☐ |  |
| 3.4 | Riktig brukers dot lyser (ikke kryssaktivering) | Kun den som snakker sin egen rad lyser | ☐ |  |
| 3.5 | Ingen knepping/crackling under normal tale | Ren lyd over 30 sek samtale | ☐ |  |
| 3.6 | PTT fungerer i Wails | Hold tast → sender; slipp → stopper | ☐ |  |
| 3.7 | PTT fungerer i nettleser (V-tast, ikke Alt) | Samme som 3.6 | ☐ |  |
| 3.8 | Voice Activation fungerer (om testet) | Sender automatisk ved tale over terskel | ☐ |  |
| 3.9 | Kanalbytte stopper/starter lyd korrekt | Bytt kanal → ingen lyd-lekkasje til gammel kanal | ☐ |  |

## 4. Chat

| # | Test | Forventet | Pass/Fail | Notat |
|---|------|-----------|-----------|-------|
| 4.1 | Melding sendt fra bruker A vises hos bruker B | Riktig brukernavn, farge, tidsstempel | ☐ |  |
| 4.2 | Melding sendt fra bruker B vises hos bruker A | Samme | ☐ |  |
| 4.3 | Melding går KUN til egen kanal | Ikke synlig i andre kanaler (test: send i `lobby`, sjekk `spill1`) | ☐ |  |
| 4.4 | URL i chat blir klikkbar lenke | Forkortet visning, åpner i ny fane | ☐ |  |
| 4.5 | XSS-test: lim inn `https://x"onmouseover="alert(1)` | **Ingen popup, lenke vises trygt escaped** | ☐ | Kritisk sikkerhetstest — kjør hver gang chat-kode endres |
| 4.6 | Tekst med spesialtegn (`<>&"'`) vises trygt | Ingen HTML tolkes som kode | ☐ |  |
| 4.7 | Historikk lastes ved kanal-bytte | Siste meldinger vises, kronologisk rekkefølge | ☐ |  |

## 5. Visuelt / UI

| # | Test | Forventet | Pass/Fail | Notat |
|---|------|-----------|-----------|-------|
| 5.1 | Innstillinger-modal åpner | Klikk på ikon → modal vises (`display:flex`) | ☐ |  |
| 5.2 | Innstillinger-modal lukker | X eller utenfor-klikk lukker den | ☐ |  |
| 5.3 | PTT-tast kan settes på nytt | Klikk felt → trykk tast → lagres | ☐ |  |
| 5.4 | Ingen emoji/uventet markdown i chat-rendering | Ren tekst + kode-blokker only | ☐ |  |
| 5.5 | Vindu kan resizes uten layout-brudd | Min 1024×600 testet | ☐ |  |
| 5.6 | Mørk theme konsistent i alle modaler | Ingen hvit "flash" eller feil kontrast | ☐ |  |

## 6. Brukeropplevelse (subjektiv, men spør eksplisitt)

| # | Spørsmål til tester | Pass/Fail | Notat |
|---|---|---|---|
| 6.1 | Følte tilkobling/login "rask" (under 3 sek til klar)? | ☐ |  |
| 6.2 | Var det noe øyeblikk av forvirring om hva som skjedde? | ☐ | Beskriv i notat hvis Fail |
| 6.3 | Føltes lyd-latensen lav nok til naturlig samtale? | ☐ |  |
| 6.4 | Var PTT-knappen/tasten intuitiv å finne? | ☐ |  |

---

## Oppsummering — fyll ut etter hver test-runde

```
Dato: __________
Commit/versjon testet: __________
Testere: __________

Antall Pass: ___ / ___
Antall Fail: ___

KRITISKE FAILS (blokkerer release):
- 

MINOR FAILS (kan vente):
- 

Konklusjon: [ ] Klar for videre bruk   [ ] Trenger fiks før neste test
```

---

## Når denne sjekklisten MÅ kjøres i sin helhet

- Etter enhver endring i `internal/ws/sfu.go` eller `fanoutLoop` (lyd-kritisk)
- Etter enhver endring i WS-auth eller kanal-håndtering (seksjon 2 og 4.3)
- Etter enhver endring i chat-rendering/escaping (seksjon 4.5 er ikke valgfri)
- Før du sier til den andre testeren "dette er klart, test det"

## Når en forkortet versjon er OK

- Ren UI-styling-endring uten lyd/chat-logikk → kun seksjon 1 + 5
- Dokumentasjons-/kommentar-only endring → kun seksjon 1.1-1.3

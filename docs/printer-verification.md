# Printer profile verification

This document tracks evidence required before a profile may be marked
`verified`. A profile being loadable or matching a model name is not proof
that its counter semantics are correct.

## Current matrix

| Model/profile | Fixture (`backend/testdata/printer-mib/`) | Real device | Current status |
| --- | --- | --- | --- |
| HP LaserJet Pro M402dn | `hp-laserjet-pro-m402dn.json` (synthetic) | Not tested | `experimental` |
| HP LaserJet Enterprise M501dn | `hp-laserjet-enterprise-m501dn.json` (synthetic) | Not tested | `experimental` |
| Brother HL-L5100DN/T | `brother-hl-l5100dn.json` (synthetic) | Not tested | `experimental` |
| Brother HL-L6210 | `brother-hl-l6210dw.json` (synthetic) | Not tested | `experimental` |
| Ricoh IM 2702 (multi-marker) | `ricoh-im-2702-multi-marker.json` (synthetic) | Not tested | `unverified` |
| Generic Printer-MIB | Synthetic join & replay tests | Not tested | `unverified` |

The repository includes synthetic `Printer-MIB.yaml` fixtures (`synthetic_fixture: true`) for automated offline replay (`FixtureClient`), OID resolution, and fast-path polling tests. Do not promote a profile to `verified` until an anonymized capture from real hardware is compared against the printer panel or configuration page.

## Fixture contract

For each physical model, collect one sanitized `snmp-debug export` JSON file
under a local, ignored fixture directory. The shared fixture must contain:

- `sysDescr`, `sysObjectID`, `sysName` and serial value masked before commit;
- the complete marker-life and marker-unit WALK results, including OID
  instances;
- the counter value shown on the printer panel or configuration page at the
  same observation time;
- model, firmware version, protocol version and collection timestamp.

Credentials, real IP addresses, serial numbers and customer identifiers must
not be committed. A fixture is acceptable only when the marker-life and
marker-unit rows join by identical instance and the selected row is explained
by the device evidence. Multiple marker rows must remain ambiguous until the
device evidence identifies the correct scope.

## Verification procedure

1. Capture a read-only export with `snmp-debug export` using an authorized
   printer and a bounded timeout.
2. Record the panel/configuration-page counter without changing printer state.
3. Compare the raw OID value, unit and instance with the fixture and profile.
4. Repeat after a known print batch, if operationally safe, and confirm the
   counter delta is plausible for the selected unit.
5. Only then change `verification_status` and add the sanitized fixture-backed
   test. Keep the original raw value and evidence reference in the review.

No real-device verification was performed as part of the current milestone.

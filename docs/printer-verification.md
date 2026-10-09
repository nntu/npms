# Printer profile verification

This document tracks evidence required before a profile may be marked
`verified`. A profile being loadable or matching a model name is not proof
that its counter semantics are correct.

## Current matrix

| Model/profile | Fixture | Real device | Current status |
| --- | --- | --- | --- |
| HP LaserJet Pro M402dn | Missing | Not tested | `experimental` |
| HP LaserJet Enterprise M501dn | Missing | Not tested | `experimental` |
| Brother HL-L5100DN/T | Missing | Not tested | `experimental` |
| Brother HL-L6210 | Missing | Not tested | `experimental` |
| Generic Printer-MIB | Synthetic join tests only | Not tested | `unverified` |

The repository currently has profile-load and exact-instance WALK join tests,
but no captured printer export. Do not add guessed vendor OIDs or promote a
profile based only on a model-name match.

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

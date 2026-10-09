# SNMP profiles

Profiles are versioned YAML declarations. They describe OIDs and interpretation
rules only; they cannot execute code, write to the database, or call commands.

Resolution precedence is:

```text
explicit assignment > exact sysObjectID > sysObjectID prefix
> vendor/model > vendor generic > generic Printer-MIB
```

An equal-priority match returns an ambiguity error. The generic profile is
`unverified` until a raw WALK and device panel/configuration-page comparison are
available. A marker life value is not automatically interpreted as physical
sheets or pages.

## External profile library

Profiles are YAML files loaded from `profiles.path` in `config.yaml`; they are
deliberately not embedded in the single executable. This allows adding or
updating a model profile without rebuilding the API/worker binary. Keep the
directory read-only for the service account and validate profiles during
startup.

The repository includes initial experimental profiles for HP LaserJet Pro
M402dn, HP LaserJet Enterprise M501dn, Brother HL-L5100DN/T and Brother
HL-L6210DW/T. They currently use the generic Printer-MIB marker-life walk and
are marked `experimental`; they do not claim verified vendor counter semantics.
Promote a profile only after collecting an anonymized SNMP walk and comparing
unit/index and counter values with the printer panel or configuration page.

During printer registration, `profile_id` may explicitly select a profile;
otherwise the server applies the precedence above from the supplied identity.
The selected profile is snapshotted in SQLite with a checksum and its counter
definitions are created atomically. A WALK counter is accepted only when its
value/unit columns produce exactly one validated marker row; multiple rows are
reported as ambiguous rather than guessed.

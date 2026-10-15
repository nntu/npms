package ingestion

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"npms/backend/internal/profile"
	"npms/backend/internal/repository"
	"npms/backend/internal/snmp"
)

// ProfileTestReport details the dry-run verification result of a profile against a device.
type ProfileTestReport struct {
	ProfileID    string              `json:"profile_id"`
	Compatible   bool                `json:"compatible"`
	FitnessScore int                 `json:"fitness_score"` // 0 to 100
	Message      string              `json:"message"`
	DurationMS   int64               `json:"duration_ms"`
	Counters     []CounterTestResult `json:"counters"`
}

// CounterTestResult details the verification result for a single counter in the profile.
type CounterTestResult struct {
	Key          string `json:"key"`
	OID          string `json:"oid"`
	Mode         string `json:"mode"`
	SemanticType string `json:"semantic_type"`
	ExpectedUnit string `json:"expected_unit"`
	ActualUnit   string `json:"actual_unit,omitempty"`
	Status       string `json:"status"` // "ok", "failed", "unsupported"
	Value        *int64 `json:"value,omitempty"`
	Instance     string `json:"instance,omitempty"`
	Error        string `json:"error,omitempty"`
}

// ProfileTester executes dry-run probes of a profile against a real or mocked SNMP device.
type ProfileTester struct {
	Resolver ConfigResolver
	Factory  ClientFactory
}

// NewProfileTester creates a new ProfileTester instance.
func NewProfileTester(resolver ConfigResolver, factory ClientFactory) (*ProfileTester, error) {
	if resolver.Store == nil || resolver.Box == nil {
		return nil, fmt.Errorf("credential resolver is required")
	}
	if factory == nil {
		return nil, fmt.Errorf("SNMP client factory is required")
	}
	return &ProfileTester{Resolver: resolver, Factory: factory}, nil
}

// TestProfileWithClient performs live SNMP reads for all counters declared in the profile using an existing SNMP client.
func (pt *ProfileTester) TestProfileWithClient(ctx context.Context, client snmp.Client, prof profile.Profile) *ProfileTestReport {
	startTime := time.Now()

	// Sort counter keys for deterministic output
	keys := make([]string, 0, len(prof.Counters))
	for k := range prof.Counters {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	report := &ProfileTestReport{
		ProfileID: prof.ID,
		Counters:  make([]CounterTestResult, 0, len(keys)),
	}

	primaryPassed := false
	primaryFound := false
	auxTotal := 0
	auxPassed := 0

	for _, key := range keys {
		def := prof.Counters[key]
		isPrimary := key == "marker_life" || key == "page_count" || def.SemanticType == "marker_life"
		if !primaryFound && (isPrimary || len(keys) == 1) {
			primaryFound = true
			isPrimary = true
		} else if !isPrimary {
			auxTotal++
		}

		res := CounterTestResult{
			Key:          key,
			OID:          def.ValueColumn,
			Mode:         def.Mode,
			SemanticType: def.SemanticType,
			ExpectedUnit: def.Unit,
			Status:       "failed",
		}

		if def.Mode == "walk" {
			// Walk counter
			life, err := client.Walk(ctx, def.ValueColumn)
			if err != nil {
				// Try fallback for primary marker_life
				if isPrimary {
					if fbVal, fbEpoch, fbErr := tryFallbackTotalCounters(ctx, client); fbErr == nil {
						res.Status = "ok"
						val := fbVal
						res.Value = &val
						res.Instance = fbEpoch
						res.ActualUnit = def.Unit
					} else {
						res.Error = fmt.Sprintf("walk failed: %v", err)
					}
				} else {
					res.Error = fmt.Sprintf("walk failed: %v", err)
				}
			} else if len(life) == 0 {
				if isPrimary {
					if fbVal, fbEpoch, fbErr := tryFallbackTotalCounters(ctx, client); fbErr == nil {
						res.Status = "ok"
						val := fbVal
						res.Value = &val
						res.Instance = fbEpoch
						res.ActualUnit = def.Unit
					} else {
						res.Error = "walk returned 0 rows"
					}
				} else {
					res.Error = "walk returned 0 rows"
				}
			} else {
				// We have walk life rows
				if def.UnitColumn != "" {
					units, _ := client.Walk(ctx, def.UnitColumn)
					rows := snmp.JoinMarkerRows(life, units, def.ValueColumn, def.UnitColumn)
					if len(rows) > 0 {
						matchedRow := rows[0]
						if def.Unit != "" {
							for _, r := range rows {
								if snmp.CounterUnitName(r.Unit) == def.Unit {
									matchedRow = r
									break
								}
							}
						}
						res.Status = "ok"
						val := matchedRow.Life
						res.Value = &val
						res.Instance = matchedRow.Instance
						res.ActualUnit = snmp.CounterUnitName(matchedRow.Unit)
					} else {
						// Device returned life rows but no matching unit column
						for _, vb := range life {
							if snmp.IsExceptionVarBind(vb) {
								continue
							}
							if v, ok := snmp.NumericValue(vb.Value); ok {
								inst, _ := snmp.TableInstance(vb.OID, def.ValueColumn)
								res.Status = "ok"
								val := v
								res.Value = &val
								res.Instance = inst
								res.ActualUnit = def.Unit
								break
							}
						}
						if res.Status != "ok" {
							if isPrimary {
								if fbVal, fbEpoch, fbErr := tryFallbackTotalCounters(ctx, client); fbErr == nil {
									res.Status = "ok"
									val := fbVal
									res.Value = &val
									res.Instance = fbEpoch
									res.ActualUnit = def.Unit
								} else {
									res.Error = "no validated marker rows returned"
								}
							} else {
								res.Error = "no validated marker rows returned"
							}
						}
					}
				} else {
					// Walk without unit column (e.g. general table)
					for _, vb := range life {
						if snmp.IsExceptionVarBind(vb) {
							continue
						}
						if v, ok := snmp.NumericValue(vb.Value); ok {
							inst, _ := snmp.TableInstance(vb.OID, def.ValueColumn)
							res.Status = "ok"
							val := v
							res.Value = &val
							res.Instance = inst
							res.ActualUnit = def.Unit
							break
						}
					}
					if res.Status != "ok" {
						res.Error = "no numeric values found in walk results"
					}
				}
			}
		} else {
			// Mode == "get"
			oidsToTry := []string{def.ValueColumn}
			if !strings.HasSuffix(def.ValueColumn, ".0") {
				oidsToTry = append(oidsToTry, def.ValueColumn+".0")
			}
			var getErr error
			var valFound bool

			for _, targetOID := range oidsToTry {
				vbs, err := client.Get(ctx, []string{targetOID})
				if err != nil {
					getErr = err
					continue
				}
				if len(vbs) > 0 && !snmp.IsExceptionVarBind(vbs[0]) {
					if num, ok := snmp.NumericValue(vbs[0].Value); ok {
						res.Status = "ok"
						val := num
						res.Value = &val
						res.Instance = "0"
						res.ActualUnit = def.Unit
						valFound = true
						break
					}
				}
			}
			if !valFound {
				if getErr != nil {
					res.Error = fmt.Sprintf("get failed: %v", getErr)
				} else {
					res.Error = "OID returned noSuchObject or non-numeric value"
				}
			}
		}

		if res.Status == "ok" {
			if isPrimary {
				primaryPassed = true
			} else {
				auxPassed++
			}
		}

		report.Counters = append(report.Counters, res)
	}

	report.DurationMS = time.Since(startTime).Milliseconds()

	// Compute FitnessScore and Compatibility
	if primaryPassed {
		score := 70
		if auxTotal > 0 {
			score += int(float64(auxPassed) / float64(auxTotal) * 30.0)
		} else {
			score = 100
		}
		report.FitnessScore = score
		report.Compatible = true
		report.Message = fmt.Sprintf("Profile tương thích tốt: đọc thành công bộ đếm trang chính (%d/%d counters đạt).", 1+auxPassed, len(keys))
	} else {
		if auxPassed > 0 {
			report.FitnessScore = 30
			report.Compatible = false
			report.Message = "Profile không tương thích: không đọc được bộ đếm số trang chính dù một số OID phụ phản hồi."
		} else {
			report.FitnessScore = 0
			report.Compatible = false
			report.Message = "Profile không tương thích: thiết bị không phản hồi bất kỳ OID bộ đếm nào."
		}
	}

	return report
}

// TestProfileWithConfig tests a profile against an SNMP device using explicit SNMP connection settings.
func (pt *ProfileTester) TestProfileWithConfig(ctx context.Context, cfg snmp.Config, prof profile.Profile) (*ProfileTestReport, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid SNMP config: %w", err)
	}
	client, err := pt.Factory(cfg)
	if err != nil {
		return nil, fmt.Errorf("connect SNMP: %w", err)
	}
	defer client.Close()

	return pt.TestProfileWithClient(ctx, client, prof), nil
}

// TestProfileForDevice tests a profile against an existing registered device endpoint.
func (pt *ProfileTester) TestProfileForDevice(ctx context.Context, deviceID string, prof profile.Profile) (*ProfileTestReport, error) {
	store, ok := pt.Resolver.Store.(DeviceCounterStore)
	if !ok {
		return nil, fmt.Errorf("device counter store is not configured")
	}
	endpoints, err := store.ListDeviceEndpoints(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("list device endpoints: %w", err)
	}
	var endpoint repository.DeviceEndpoint
	for _, ep := range endpoints {
		if ep.IsPrimary {
			endpoint = ep
			break
		}
	}
	if endpoint.ID == "" && len(endpoints) > 0 {
		endpoint = endpoints[0]
	}
	if endpoint.ID == "" {
		return nil, fmt.Errorf("device %s has no configured SNMP endpoints", deviceID)
	}

	cfg, err := pt.Resolver.Resolve(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("resolve endpoint credentials: %w", err)
	}

	return pt.TestProfileWithConfig(ctx, cfg, prof)
}

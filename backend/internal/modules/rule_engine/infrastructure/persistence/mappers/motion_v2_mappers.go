package mappers

import (
	"encoding/json"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// Columnas jsonb de motion-v2 (rules_hit, history, long_hold) y los datos
// nuevos de los motivos. La vuelta reconstruye lo que necesita el dominio.

// rulesHitJSON: NULL en NO_CALCULABLE; [] si ningún umbral cuenta.
func rulesHitJSON(e entities.MotionEvaluation) *string {
	if e.Status != entities.MotionStatusCalculated {
		return nil
	}
	hits := e.RulesHit
	if hits == nil {
		hits = []entities.MotionRuleHit{}
	}
	b, _ := json.Marshal(hits)
	s := string(b)
	return &s
}

func rulesHitFromJSON(raw *string) []entities.MotionRuleHit {
	if raw == nil {
		return nil
	}
	var out []entities.MotionRuleHit
	if err := json.Unmarshal([]byte(*raw), &out); err != nil {
		return nil
	}
	return out
}

func historyJSON(h *entities.MotionHistory) *string {
	if h == nil {
		return nil
	}
	b, _ := json.Marshal(h)
	s := string(b)
	return &s
}

func historyFromJSON(raw *string) *entities.MotionHistory {
	if raw == nil {
		return nil
	}
	var h entities.MotionHistory
	if err := json.Unmarshal([]byte(*raw), &h); err != nil {
		return nil
	}
	return &h
}

func longHoldJSON(h map[string]int) string {
	if len(h) == 0 {
		return "{}"
	}
	b, _ := json.Marshal(h)
	return string(b)
}

func longHoldFromJSON(raw string) map[string]int {
	var h map[string]int
	if err := json.Unmarshal([]byte(raw), &h); err != nil || len(h) == 0 {
		return nil
	}
	return h
}

// v2ReasonFromData completa los campos motion-v2 del motivo desde su data.
func v2ReasonFromData(r *entities.MotionReason, d map[string]any) {
	hit := func(spanKey string) *entities.MotionRuleHit {
		if _, ok := d["axis"].(string); !ok {
			return nil
		}
		return &entities.MotionRuleHit{
			Axis: str(d["axis"]), Delta: num(d["delta"]), SpanHours: int(num(d[spanKey])),
			WindowHours: int(num(d["window_hours"])), Level: entities.Level(str(d["level"])), Deg: num(d["deg"]),
			Since: timePtr(d["since"]),
		}
	}
	switch r.Code {
	case entities.ReasonMovAbrupt:
		// En MOV_ABRUPT "hours" es el span del umbral, no Hours.
		r.Hours = 0
		r.Hit, r.AmarilloDeg = hit("hours"), numPtr(d["amarillo_deg"])
		if r.Hit != nil {
			r.Hit.Kind = entities.MotionRuleShort
		}
	case entities.ReasonMovCumulative:
		r.Hit, r.AmarilloDeg = hit("span_hours"), numPtr(d["amarillo_deg"])
		if r.Hit != nil {
			r.Hit.Kind = entities.MotionRuleLong
		}
		r.Steps, r.Pattern, r.RatePerDay3d = int(num(d["steps"])), str(d["pattern"]), numPtr(d["rate_per_day_3d"])
		if list, ok := d["step_list"].([]any); ok {
			for _, x := range list {
				m, _ := x.(map[string]any)
				if at := timePtr(m["at"]); at != nil {
					r.StepList = append(r.StepList, entities.MotionStep{At: *at, Delta: num(m["delta"])})
				}
			}
		}
	case entities.ReasonMovLevelUp:
		r.FromLevel, r.ToLevel = entities.Level(str(d["from_level"])), entities.Level(str(d["to_level"]))
		r.SentFrom, r.SentTo = entities.Level(str(d["sent_from"])), entities.Level(str(d["sent_to"]))
		if r.Hit = hit("span_hours"); r.Hit != nil {
			// kind y level no viajan: se derivan de la ventana y de to_level.
			r.Hit.Level, r.Hit.Kind = r.ToLevel, entities.MotionRuleShort
			if r.Hit.WindowHours > entities.MotionShortMaxHours {
				r.Hit.Kind = entities.MotionRuleLong
			}
		}
	case entities.ReasonMovDaily:
		r.NoData, _ = d["no_data"].(bool)
		r.CalculatedHours24h, r.Day = int(num(d["calculated_hours_24h"])), int(num(d["day"]))
		r.Level, r.Sent = entities.Level(str(d["level"])), entities.Level(str(d["sent"]))
		r.DP24h, r.DR24h = numPtr(d["dp_24h"]), numPtr(d["dr_24h"])
		r.DPTotal, r.DRTotal = numPtr(d["dp_total"]), numPtr(d["dr_total"])
	case entities.ReasonMovNewPosition:
		r.DPTotal, r.DRTotal = numPtr(d["dp_total"]), numPtr(d["dr_total"])
	case entities.ReasonMovStill:
		r.DPTotal, r.DRTotal = numPtr(d["dp_total"]), numPtr(d["dr_total"])
		r.Level, r.Sent = entities.Level(str(d["max_level"])), entities.Level(str(d["max_sent"]))
		r.LongEpisode, _ = d["long_episode"].(bool)
	}
}
